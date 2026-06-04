// assert-queue verifies FIFO queue ordering for a sale (zero inversions).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/arkin/traffic/internal/config"
	"github.com/arkin/traffic/internal/db"
	"github.com/arkin/traffic/internal/redisclient"
	"github.com/arkin/traffic/internal/waitingroom"
)

func main() {
	saleID := flag.String("sale", "", "sale UUID")
	flag.Parse()
	if *saleID == "" {
		fmt.Fprintln(os.Stderr, "usage: assert-queue -sale <uuid>")
		os.Exit(2)
	}

	cfg := config.Load()
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL, cfg.DBMaxConns, cfg.DBMinConns)
	if err != nil {
		fatal(err)
	}
	defer pool.Close()

	rdb, err := redisclient.Connect(ctx, cfg.RedisURL)
	if err != nil {
		fatal(err)
	}
	defer rdb.Close()

	queue := waitingroom.New(pool, rdb, cfg)
	entries, err := queue.QueueEntriesForAssert(ctx, *saleID)
	if err != nil {
		fatal(err)
	}

	if len(entries) == 0 {
		fmt.Println("no queue entries")
		os.Exit(0)
	}

	type row struct {
		Position int64
		Joined   int64 // unix nano for sort
		UserID   string
	}
	rows := make([]row, len(entries))
	for i, e := range entries {
		rows[i] = row{Position: e.Position, Joined: e.JoinedAt.UnixNano(), UserID: e.UserID}
	}

	byJoin := make([]row, len(rows))
	copy(byJoin, rows)
	sort.Slice(byJoin, func(i, j int) bool {
		if byJoin[i].Joined != byJoin[j].Joined {
			return byJoin[i].Joined < byJoin[j].Joined
		}
		return byJoin[i].Position < byJoin[j].Position
	})

	inversions := 0
	for i := 1; i < len(byJoin); i++ {
		if byJoin[i].Position < byJoin[i-1].Position {
			inversions++
			fmt.Printf("INVERSION: user %s pos %d joined after user %s pos %d\n",
				byJoin[i].UserID, byJoin[i].Position,
				byJoin[i-1].UserID, byJoin[i-1].Position)
		}
	}

	byPos := make([]row, len(rows))
	copy(byPos, rows)
	sort.Slice(byPos, func(i, j int) bool { return byPos[i].Position < byPos[j].Position })

	for i := 1; i < len(byPos); i++ {
		if byPos[i].Joined < byPos[i-1].Joined {
			inversions++
			fmt.Printf("LATE_AHEAD: position %d joined before position %d but has later timestamp\n",
				byPos[i].Position, byPos[i-1].Position)
		}
	}

	fmt.Printf("entries=%d inversions=%d\n", len(entries), inversions)
	if inversions > 0 {
		os.Exit(1)
	}
	fmt.Println("PASS: queue order is fair (zero inversions)")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
