package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/arkin/traffic/internal/config"
)

var (
	ErrSeatUnavailable = errors.New("seat unavailable")
	ErrHoldNotFound    = errors.New("hold not found")
	ErrHoldExpired     = errors.New("hold expired")
)

type Seat struct {
	ID      string `json:"id"`
	Section string `json:"section"`
	Row     int    `json:"row"`
	Num     int    `json:"num"`
	Status  string `json:"status"`
}

type Service struct {
	db  *pgxpool.Pool
	cfg config.Config
}

func New(db *pgxpool.Pool, cfg config.Config) *Service {
	return &Service{db: db, cfg: cfg}
}

func (s *Service) ListAvailable(ctx context.Context, saleID string, limit int) ([]Seat, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, section, row_num, seat_num, status FROM seats
		WHERE sale_id = $1 AND status = 'available'
		ORDER BY section, row_num, seat_num LIMIT $2`, saleID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var seats []Seat
	for rows.Next() {
		var seat Seat
		if err := rows.Scan(&seat.ID, &seat.Section, &seat.Row, &seat.Num, &seat.Status); err != nil {
			return nil, err
		}
		seats = append(seats, seat)
	}
	return seats, rows.Err()
}

func (s *Service) CreateSeats(ctx context.Context, saleID string, total int) error {
	section := "GA"
	perRow := 50
	rows := (total + perRow - 1) / perRow
	for r := 1; r <= rows; r++ {
		for n := 1; n <= perRow; n++ {
			if (r-1)*perRow+n > total {
				break
			}
			_, err := s.db.Exec(ctx, `
				INSERT INTO seats (sale_id, section, row_num, seat_num, status)
				VALUES ($1, $2, $3, $4, 'available')
				ON CONFLICT DO NOTHING`, saleID, section, r, n)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type HoldResult struct {
	HoldID    string    `json:"hold_id"`
	SeatIDs   []string  `json:"seat_ids"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Service) Hold(ctx context.Context, saleID, userID, idempotencyKey string, seatIDs []string) (*HoldResult, error) {
	if len(seatIDs) == 0 || len(seatIDs) > 4 {
		return nil, errors.New("invalid seat count")
	}

	var existing HoldResult
	var seatJSON []byte
	err := s.db.QueryRow(ctx, `
		SELECT id, seat_ids, expires_at FROM holds
		WHERE sale_id = $1 AND user_id = $2 AND idempotency_key = $3`,
		saleID, userID, idempotencyKey).Scan(&existing.HoldID, &seatJSON, &existing.ExpiresAt)
	if err == nil {
		_ = json.Unmarshal(seatJSON, &existing.SeatIDs)
	}
	if err == nil {
		if time.Now().After(existing.ExpiresAt) {
			return nil, ErrHoldExpired
		}
		return &existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// max 1 active hold per user per sale
	var active int
	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM holds
		WHERE sale_id = $1 AND user_id = $2 AND expires_at > NOW()`, saleID, userID).Scan(&active)
	if active > 0 {
		return nil, errors.New("active hold already exists")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	for _, seatID := range seatIDs {
		tag, err := tx.Exec(ctx, `
			UPDATE seats SET status = 'held', version = version + 1
			WHERE id = $1 AND sale_id = $2 AND status = 'available'`, seatID, saleID)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			return nil, ErrSeatUnavailable
		}
	}

	expires := time.Now().Add(s.cfg.HoldTTL)
	holdID := uuid.New().String()
	seatJSON, err = json.Marshal(seatIDs)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO holds (id, sale_id, user_id, seat_ids, idempotency_key, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		holdID, saleID, userID, seatJSON, idempotencyKey, expires)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &HoldResult{HoldID: holdID, SeatIDs: seatIDs, ExpiresAt: expires}, nil
}

func (s *Service) ReleaseExpired(ctx context.Context) (int64, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, seat_ids FROM holds WHERE expires_at < NOW() LIMIT 100`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var count int64
	for rows.Next() {
		var holdID string
		var seatJSON []byte
		if err := rows.Scan(&holdID, &seatJSON); err != nil {
			return count, err
		}
		var seatIDs []string
		_ = json.Unmarshal(seatJSON, &seatIDs)
		tx, err := s.db.Begin(ctx)
		if err != nil {
			return count, err
		}
		for _, sid := range seatIDs {
			_, _ = tx.Exec(ctx, `
				UPDATE seats SET status = 'available', version = version + 1
				WHERE id = $1 AND status = 'held'`, sid)
		}
		_, _ = tx.Exec(ctx, `DELETE FROM holds WHERE id = $1`, holdID)
		if err := tx.Commit(ctx); err == nil {
			count++
		} else {
			tx.Rollback(ctx)
		}
	}
	return count, nil
}
