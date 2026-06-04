package orders

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/arkin/traffic/internal/inventory"
	"github.com/arkin/traffic/internal/metrics"
)

var (
	ErrTicketCapReached = errors.New("ticket purchase cap reached")
	ErrTransferBlocked  = errors.New("ticket transfers are not permitted")
)

type Service struct {
	db  *pgxpool.Pool
	inv *inventory.Service
}

func New(db *pgxpool.Pool, inv *inventory.Service) *Service {
	return &Service{db: db, inv: inv}
}

type CheckoutResult struct {
	OrderID  string   `json:"order_id"`
	TicketIDs []string `json:"ticket_ids"`
	Status   string   `json:"status"`
}

func ownerHash(userID string) string {
	h := sha256.Sum256([]byte(userID))
	return hex.EncodeToString(h[:8])
}

func (s *Service) Checkout(ctx context.Context, saleID, userID, idempotencyKey, holdID string) (*CheckoutResult, error) {
	var existing CheckoutResult
	err := s.db.QueryRow(ctx, `
		SELECT id, status FROM orders
		WHERE sale_id = $1 AND user_id = $2 AND idempotency_key = $3`,
		saleID, userID, idempotencyKey).Scan(&existing.OrderID, &existing.Status)
	if err == nil {
		metrics.Checkouts.WithLabelValues(saleID, "idempotent").Inc()
		return &existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	var maxPerUser int
	_ = s.db.QueryRow(ctx, `SELECT max_tickets_per_user FROM sales WHERE id = $1`, saleID).Scan(&maxPerUser)
	if maxPerUser <= 0 {
		maxPerUser = 4
	}

	var owned int
	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM tickets WHERE sale_id = $1 AND user_id = $2 AND status = 'active'`,
		saleID, userID).Scan(&owned)

	var seatJSON []byte
	err = s.db.QueryRow(ctx, `
		SELECT seat_ids FROM holds
		WHERE id = $1 AND user_id = $2 AND sale_id = $3 AND expires_at > NOW()`,
		holdID, userID, saleID).Scan(&seatJSON)
	if err != nil {
		metrics.Checkouts.WithLabelValues(saleID, "rejected").Inc()
		return nil, inventory.ErrHoldNotFound
	}
	var seatIDs []string
	if err := json.Unmarshal(seatJSON, &seatIDs); err != nil {
		return nil, err
	}
	if owned+len(seatIDs) > maxPerUser {
		metrics.Checkouts.WithLabelValues(saleID, "cap_exceeded").Inc()
		return nil, ErrTicketCapReached
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	orderID := uuid.New().String()
	total := len(seatIDs) * 5000
	_, err = tx.Exec(ctx, `
		INSERT INTO orders (id, sale_id, user_id, hold_id, idempotency_key, status, total_cents)
		VALUES ($1, $2, $3, $4, $5, 'confirmed', $6)`,
		orderID, saleID, userID, holdID, idempotencyKey, total)
	if err != nil {
		return nil, err
	}

	hash := ownerHash(userID)
	var ticketIDs []string
	for _, seatID := range seatIDs {
		tid := uuid.New().String()
		tag, err := tx.Exec(ctx, `
			UPDATE seats SET status = 'sold', version = version + 1
			WHERE id = $1 AND sale_id = $2 AND status = 'held'`, seatID, saleID)
		if err != nil || tag.RowsAffected() == 0 {
			return nil, inventory.ErrSeatUnavailable
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO tickets (id, order_id, sale_id, user_id, seat_id, owner_hash, status)
			VALUES ($1, $2, $3, $4, $5, $6, 'active')`,
			tid, orderID, saleID, userID, seatID, hash)
		if err != nil {
			return nil, err
		}
		ticketIDs = append(ticketIDs, tid)
	}

	_, _ = tx.Exec(ctx, `DELETE FROM holds WHERE id = $1`, holdID)
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	metrics.Checkouts.WithLabelValues(saleID, "success").Inc()
	return &CheckoutResult{OrderID: orderID, TicketIDs: ticketIDs, Status: "confirmed"}, nil
}

func (s *Service) Transfer(ctx context.Context) error {
	return ErrTransferBlocked
}

func (s *Service) Stats(ctx context.Context, saleID string) (map[string]interface{}, error) {
	var tickets, orders, buyers int
	_ = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM tickets WHERE sale_id = $1`, saleID).Scan(&tickets)
	_ = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE sale_id = $1 AND status = 'confirmed'`, saleID).Scan(&orders)
	_ = s.db.QueryRow(ctx, `SELECT COUNT(DISTINCT user_id) FROM tickets WHERE sale_id = $1`, saleID).Scan(&buyers)
	return map[string]interface{}{
		"tickets_sold":   tickets,
		"orders":         orders,
		"unique_buyers":  buyers,
		"scalper_signal": fmt.Sprintf("buyers/orders ratio=%.2f", float64(buyers)/float64(max(orders, 1))),
	}, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
