package dashboard

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/arkin/traffic/internal/redisclient"
	"github.com/arkin/traffic/internal/simulation"
)

type LiveStats struct {
	Timestamp       time.Time              `json:"timestamp"`
	SaleID          string                 `json:"sale_id"`
	QueueDepth      int64                  `json:"queue_depth"`
	NextAdmitPos    int64                  `json:"next_admit_position"`
	AdmittedCount   int64                  `json:"admitted_count"`
	TicketsSold     int                    `json:"tickets_sold"`
	UniqueBuyers    int                    `json:"unique_buyers"`
	ActiveHolds     int                    `json:"active_holds"`
	AvailableSeats  int                    `json:"available_seats"`
	Simulation      simulation.Stats       `json:"simulation"`
	Health          string                 `json:"health"`
	ErrorRatePct    float64                `json:"error_rate_pct"`
	RequestsPerSec  float64                `json:"requests_per_sec_approx"`
	LastLatencyMs   int64                  `json:"last_latency_ms"`
}

type Service struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}

func New(db *pgxpool.Pool, rdb *redis.Client) *Service {
	return &Service{db: db, rdb: rdb}
}

func (s *Service) Live(ctx context.Context, saleID string, sim *simulation.Engine) (*LiveStats, error) {
	st := &LiveStats{
		Timestamp: time.Now().UTC(),
		SaleID:    saleID,
		Health:    "healthy",
	}
	if sim != nil {
		st.Simulation = sim.StatsSnapshot()
		snap := st.Simulation
		if snap.RequestsTotal > 0 {
			st.ErrorRatePct = float64(snap.ErrorsTotal) / float64(snap.RequestsTotal) * 100
		}
		st.LastLatencyMs = snap.LastLatencyMs
		if snap.RequestsTotal > 0 {
			elapsed := time.Since(simStart(sim)).Seconds()
			if elapsed < 1 {
				elapsed = 1
			}
			st.RequestsPerSec = float64(snap.RequestsTotal) / elapsed
		}
		if st.RequestsPerSec > 500 {
			st.Health = "under_load"
		}
		if st.ErrorRatePct > 5 {
			st.Health = "degraded"
		}
	}

	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(position), 0) FROM queue_entries WHERE sale_id = $1`, saleID).Scan(&st.QueueDepth)

	keys := redisclient.Keys(saleID)
	next, err := s.rdb.Get(ctx, keys.NextAdmit).Int64()
	if err == redis.Nil {
		next = 0
	}
	st.NextAdmitPos = next

	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM queue_entries WHERE sale_id = $1 AND admitted_at IS NOT NULL`, saleID).Scan(&st.AdmittedCount)

	_ = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM tickets WHERE sale_id = $1`, saleID).Scan(&st.TicketsSold)
	_ = s.db.QueryRow(ctx, `SELECT COUNT(DISTINCT user_id) FROM tickets WHERE sale_id = $1`, saleID).Scan(&st.UniqueBuyers)
	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM holds WHERE sale_id = $1 AND expires_at > NOW()`, saleID).Scan(&st.ActiveHolds)
	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM seats WHERE sale_id = $1 AND status = 'available'`, saleID).Scan(&st.AvailableSeats)

	return st, nil
}

func simStart(sim *simulation.Engine) time.Time {
	return sim.StartedAt()
}
