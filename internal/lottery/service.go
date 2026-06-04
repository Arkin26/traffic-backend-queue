package lottery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) PreRegister(ctx context.Context, saleID, userID string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO pre_registrations (sale_id, user_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, saleID, userID)
	return err
}

func (s *Service) Draw(ctx context.Context, saleID string, winners int) (int, error) {
	rows, err := s.db.Query(ctx, `
		SELECT user_id FROM pre_registrations WHERE sale_id = $1
		ORDER BY random() LIMIT $2`, saleID, winners)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return count, err
		}
		code := uuid.New().String()
		hash := sha256.Sum256([]byte(code))
		codeHash := hex.EncodeToString(hash[:])
		_, err := s.db.Exec(ctx, `
			INSERT INTO access_codes (sale_id, user_id, code_hash) VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING`, saleID, userID, codeHash)
		if err != nil {
			return count, err
		}
		count++
	}
	_, _ = s.db.Exec(ctx, `UPDATE sales SET phase = 'presale' WHERE id = $1`, saleID)
	return count, rows.Err()
}

func (s *Service) ValidateAccessCode(ctx context.Context, saleID, userID, code string) (bool, error) {
	hash := sha256.Sum256([]byte(code))
	codeHash := hex.EncodeToString(hash[:])
	var id string
	err := s.db.QueryRow(ctx, `
		SELECT id FROM access_codes
		WHERE sale_id = $1 AND user_id = $2 AND code_hash = $3 AND used_at IS NULL`,
		saleID, userID, codeHash).Scan(&id)
	if err != nil {
		return false, nil
	}
	_, _ = s.db.Exec(ctx, `UPDATE access_codes SET used_at = NOW() WHERE id = $1`, id)
	return true, nil
}

func (s *Service) OpenGeneralSale(ctx context.Context, saleID string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE sales SET phase = 'general_sale', opens_at = NOW() WHERE id = $1`, saleID)
	return err
}
