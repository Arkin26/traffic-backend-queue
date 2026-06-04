package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, url string, maxConns, minConns int) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	if maxConns <= 0 {
		maxConns = 50
	}
	if minConns <= 0 {
		minConns = 5
	}
	cfg.MaxConns = int32(maxConns)
	cfg.MinConns = int32(minConns)
	return pgxpool.NewWithConfig(ctx, cfg)
}
