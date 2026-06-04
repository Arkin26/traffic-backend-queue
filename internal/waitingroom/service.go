package waitingroom

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/arkin/traffic/internal/auth"
	"github.com/arkin/traffic/internal/config"
	"github.com/arkin/traffic/internal/metrics"
	"github.com/arkin/traffic/internal/redisclient"
)

var (
	ErrSaleNotOpen     = errors.New("sale not open for queue")
	ErrWaitingRoomFull = errors.New("waiting room at capacity")
	ErrNotInQueue      = errors.New("not in queue")
)

type Status struct {
	Position         int64  `json:"position"`
	AheadCount       int64  `json:"ahead_count"`
	EstimatedWaitSec int64  `json:"estimated_wait_sec"`
	Admitted         bool   `json:"admitted"`
	AdmissionToken   string `json:"admission_token,omitempty"`
	RetryAfterMs     int64  `json:"retry_after_ms"`
	Message          string `json:"message"`
}

type Service struct {
	db     *pgxpool.Pool
	rdb    *redis.Client
	cfg    config.Config
	secret string
}

func New(db *pgxpool.Pool, rdb *redis.Client, cfg config.Config) *Service {
	return &Service{db: db, rdb: rdb, cfg: cfg, secret: cfg.JWTSecret}
}

type saleInfo struct {
	Phase          string
	OpensAt        time.Time
	AdmitPerMinute int
	WaitingRoomCap int
}

func (s *Service) getSale(ctx context.Context, saleID string) (*saleInfo, error) {
	var info saleInfo
	err := s.db.QueryRow(ctx, `
		SELECT phase::text, opens_at, admit_per_minute, waiting_room_cap
		FROM sales WHERE id = $1`, saleID).Scan(
		&info.Phase, &info.OpensAt, &info.AdmitPerMinute, &info.WaitingRoomCap)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (s *Service) saleOpenForQueue(info *saleInfo) bool {
	now := time.Now().UTC()
	if now.Before(info.OpensAt) {
		return false
	}
	return info.Phase == "general_sale" || info.Phase == "presale"
}

// Join assigns position once; idempotent for same user.
func (s *Service) Join(ctx context.Context, saleID, userID string) (int64, bool, error) {
	info, err := s.getSale(ctx, saleID)
	if err != nil {
		return 0, false, err
	}
	if !s.saleOpenForQueue(info) {
		metrics.QueueJoins.WithLabelValues(saleID, "rejected_closed").Inc()
		return 0, false, ErrSaleNotOpen
	}

	var existing int64
	err = s.db.QueryRow(ctx, `
		SELECT position FROM queue_entries WHERE sale_id = $1 AND user_id = $2`,
		saleID, userID).Scan(&existing)
	if err == nil {
		metrics.QueueJoins.WithLabelValues(saleID, "idempotent").Inc()
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, err
	}

	keys := redisclient.Keys(saleID)
	currentLen, err := s.rdb.Get(ctx, keys.QueueSeq).Int64()
	if err != nil && err != redis.Nil {
		return 0, false, err
	}
	if err == nil && int(currentLen) >= info.WaitingRoomCap {
		metrics.QueueJoins.WithLabelValues(saleID, "rejected_full").Inc()
		return 0, false, ErrWaitingRoomFull
	}

	position, err := s.rdb.Incr(ctx, keys.QueueSeq).Result()
	if err != nil {
		return 0, false, err
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO queue_entries (sale_id, user_id, position, joined_at)
		VALUES ($1, $2, $3, NOW())`,
		saleID, userID, position)
	if err != nil {
		// unique violation = race, return existing
		err2 := s.db.QueryRow(ctx, `
			SELECT position FROM queue_entries WHERE sale_id = $1 AND user_id = $2`,
			saleID, userID).Scan(&existing)
		if err2 == nil {
			return existing, false, nil
		}
		return 0, false, err
	}

	metrics.QueueJoins.WithLabelValues(saleID, "new").Inc()
	metrics.QueueDepth.WithLabelValues(saleID).Set(float64(position))
	return position, true, nil
}

func (s *Service) effectiveAdmitPosition(position int64) int64 {
	return position
}

func (s *Service) Status(ctx context.Context, saleID, userID string) (*Status, error) {
	var position int64
	var admittedAt *time.Time
	err := s.db.QueryRow(ctx, `
		SELECT position, admitted_at FROM queue_entries
		WHERE sale_id = $1 AND user_id = $2`, saleID, userID).Scan(&position, &admittedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotInQueue
		}
		return nil, err
	}

	info, err := s.getSale(ctx, saleID)
	if err != nil {
		return nil, err
	}

	keys := redisclient.Keys(saleID)
	nextAdmit, err := s.rdb.Get(ctx, keys.NextAdmit).Int64()
	if err == redis.Nil {
		nextAdmit = 0
	} else if err != nil {
		return nil, err
	}

	effectivePos := s.effectiveAdmitPosition(position)
	ahead := effectivePos - nextAdmit
	if ahead < 0 {
		ahead = 0
	}

	admitted := admittedAt != nil || effectivePos <= nextAdmit
	st := &Status{
		Position:         position,
		AheadCount:       ahead,
		EstimatedWaitSec: (ahead * 60) / int64(max(info.AdmitPerMinute, 1)),
		Admitted:         admitted,
		RetryAfterMs:     3000,
		Message:          "Refreshing does not change your place in line.",
	}

	if admitted {
		if admittedAt == nil {
			_, _ = s.db.Exec(ctx, `
				UPDATE queue_entries SET admitted_at = NOW()
				WHERE sale_id = $1 AND user_id = $2 AND admitted_at IS NULL`,
				saleID, userID)
			metrics.Admissions.WithLabelValues(saleID).Inc()
		}
		jti := uuid.New().String()
		token, err := auth.IssueAdmissionToken(s.secret, userID, saleID, jti, s.cfg.AdmissionTTL)
		if err != nil {
			return nil, err
		}
		st.AdmissionToken = token
	}

	return st, nil
}

// AdvanceAdmission moves the admit window (called by worker).
func (s *Service) AdvanceAdmission(ctx context.Context, saleID string, count int64) error {
	keys := redisclient.Keys(saleID)
	return s.rdb.IncrBy(ctx, keys.NextAdmit, count).Err()
}

// QueueEntriesForAssert returns entries for testing.
func (s *Service) QueueEntriesForAssert(ctx context.Context, saleID string) ([]struct {
	UserID   string
	Position int64
	JoinedAt time.Time
}, error) {
	rows, err := s.db.Query(ctx, `
		SELECT user_id, position, joined_at FROM queue_entries
		WHERE sale_id = $1 ORDER BY position ASC`, saleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		UserID   string
		Position int64
		JoinedAt time.Time
	}
	for rows.Next() {
		var e struct {
			UserID   string
			Position int64
			JoinedAt time.Time
		}
		if err := rows.Scan(&e.UserID, &e.Position, &e.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
