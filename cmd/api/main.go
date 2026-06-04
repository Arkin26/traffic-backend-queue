package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/arkin/traffic/internal/antibot"
	"github.com/arkin/traffic/internal/config"
	"github.com/arkin/traffic/internal/db"
	"github.com/arkin/traffic/internal/httpapi"
	"github.com/arkin/traffic/internal/dashboard"
	"github.com/arkin/traffic/internal/loadtest"
	"github.com/arkin/traffic/internal/inventory"
	"github.com/arkin/traffic/internal/orders"
	"github.com/arkin/traffic/internal/simulation"
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
	ord := orders.New(pool, inv)
	limit := antibot.New(rdb)
	sim := simulation.New(cfg.SimulationBaseURL, cfg.AdminAPIKey)
	dash := dashboard.New(pool, rdb)
	jobsDir, resultsDir := loadtest.DefaultDirs()
	ltStore := loadtest.NewStore(jobsDir, resultsDir)
	ltLimits := loadtest.Limits{
		MaxVUs:        cfg.LoadtestMaxVUs,
		MaxQueueDepth: cfg.LoadtestMaxQueue,
		MaxConcurrent: cfg.LoadtestMaxConcurrent,
		MaxSteady:     time.Duration(cfg.LoadtestMaxSteadySec) * time.Second,
		DefaultVUs:    cfg.LoadtestDefaultVUs,
	}
	lt := loadtest.NewServiceWithLimits(ltStore, ltLimits)

	srv := httpapi.NewServer(cfg, pool, rdb, queue, inv, ord, limit, sim, dash, lt)

	if cfg.DemoAutoSetup {
		go func() {
			time.Sleep(2 * time.Second)
			if id, err := sim.SetupDemo(ctx); err == nil {
				log.Printf("demo sale ready: %s", id)
			}
		}()
	}
	handler := srv.Routes()

	httpSrv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		log.Printf("api listening on %s", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}
