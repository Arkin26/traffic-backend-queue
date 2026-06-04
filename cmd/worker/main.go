package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/arkin/traffic/internal/config"
	"github.com/arkin/traffic/internal/db"
	"github.com/arkin/traffic/internal/inventory"
	"github.com/arkin/traffic/internal/redisclient"
	"github.com/arkin/traffic/internal/waitingroom"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL, cfg.DBMaxConns, cfg.DBMinConns)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	rdb, err := redisclient.Connect(ctx, cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer rdb.Close()

	queue := waitingroom.New(pool, rdb, cfg)
	inv := inventory.New(pool, cfg)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	log.Println("worker started")
	for {
		select {
		case <-stop:
			log.Println("worker stopping")
			return
		case <-ticker.C:
			runAdmission(ctx, pool, queue)
			if n, err := inv.ReleaseExpired(ctx); err != nil {
				log.Printf("hold release error: %v", err)
			} else if n > 0 {
				log.Printf("released %d expired holds", n)
			}
		}
	}
}

func runAdmission(ctx context.Context, pool *pgxpool.Pool, queue *waitingroom.Service) {
	rows, err := pool.Query(ctx, `
		SELECT id, admit_per_minute FROM sales
		WHERE phase IN ('general_sale', 'presale') AND ends_at > NOW()`)
	if err != nil {
		log.Printf("admission query: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var saleID string
		var admitPerMin int
		if err := rows.Scan(&saleID, &admitPerMin); err != nil {
			continue
		}
		// drip: admit_per_minute / 12 per 5-second tick
		batch := int64(admitPerMin / 12)
		if batch < 1 {
			batch = 1
		}
		if err := queue.AdvanceAdmission(ctx, saleID, batch); err != nil {
			log.Printf("admit sale %s: %v", saleID, err)
		}
	}
}
