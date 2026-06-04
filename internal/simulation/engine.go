package simulation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Scenario toggles — map to BookMyShow failure modes (no coupons/lottery).
type Config struct {
	LaunchSpike     bool `json:"launch_spike"`
	BackdoorRace    bool `json:"backdoor_race"`
	LateSurge       bool `json:"late_surge"`
	RefreshStorm    bool `json:"refresh_storm"`
	ReconnectChurn  bool `json:"reconnect_churn"`
	ScalperBots     bool `json:"scalper_bots"`
	MixedRealistic  bool `json:"mixed_realistic"`
}

type Stats struct {
	RequestsTotal   int64            `json:"requests_total"`
	ErrorsTotal     int64            `json:"errors_total"`
	BlockedTotal    int64            `json:"blocked_total"`
	LastLatencyMs   int64            `json:"last_latency_ms"`
	ActiveScenarios []string         `json:"active_scenarios"`
	Config          Config           `json:"config"`
	PerScenario     map[string]int64 `json:"per_scenario"`
}

type Engine struct {
	mu       sync.RWMutex
	cfg      Config
	baseURL  string
	adminKey string
	saleID   string
	client   *http.Client
	stop     context.CancelFunc
	stats      Stats
	perScn     map[string]*atomic.Int64
	startedAt  time.Time
}

func New(baseURL, adminKey string) *Engine {
	return &Engine{
		baseURL:   baseURL,
		adminKey:  adminKey,
		client:    &http.Client{Timeout: 10 * time.Second},
		startedAt: time.Now(),
		perScn: map[string]*atomic.Int64{
			"launch_spike":    new(atomic.Int64),
			"backdoor_race":   new(atomic.Int64),
			"late_surge":      new(atomic.Int64),
			"refresh_storm":   new(atomic.Int64),
			"reconnect_churn": new(atomic.Int64),
			"scalper_bots":    new(atomic.Int64),
			"mixed_realistic": new(atomic.Int64),
		},
	}
}

func (e *Engine) SetSaleID(id string) {
	e.mu.Lock()
	e.saleID = id
	e.mu.Unlock()
}

func (e *Engine) SaleID() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.saleID
}

func (e *Engine) StartedAt() time.Time {
	return e.startedAt
}

func (e *Engine) Update(c Config) {
	e.mu.Lock()
	e.cfg = c
	e.mu.Unlock()
	e.restartWorkers()
}

func (e *Engine) GetConfig() Config {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cfg
}

func (e *Engine) StatsSnapshot() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := e.stats
	s.Config = e.cfg
	s.PerScenario = make(map[string]int64)
	for k, v := range e.perScn {
		s.PerScenario[k] = v.Load()
	}
	s.ActiveScenarios = activeNames(e.cfg)
	return s
}

func activeNames(c Config) []string {
	var names []string
	if c.LaunchSpike {
		names = append(names, "launch_spike")
	}
	if c.BackdoorRace {
		names = append(names, "backdoor_race")
	}
	if c.LateSurge {
		names = append(names, "late_surge")
	}
	if c.RefreshStorm {
		names = append(names, "refresh_storm")
	}
	if c.ReconnectChurn {
		names = append(names, "reconnect_churn")
	}
	if c.ScalperBots {
		names = append(names, "scalper_bots")
	}
	if c.MixedRealistic {
		names = append(names, "mixed_realistic")
	}
	return names
}

func (e *Engine) restartWorkers() {
	if e.stop != nil {
		e.stop()
		e.stop = nil
	}
	e.mu.RLock()
	cfg := e.cfg
	saleID := e.saleID
	e.mu.RUnlock()
	if saleID == "" {
		return
	}
	if !anyOn(cfg) {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.stop = cancel
	if cfg.LaunchSpike || cfg.MixedRealistic {
		go e.loop(ctx, "launch_spike", 50*time.Millisecond, func() { e.doLaunchSpike(saleID) })
	}
	if cfg.BackdoorRace || cfg.MixedRealistic {
		go e.loop(ctx, "backdoor_race", 200*time.Millisecond, func() { e.doBackdoor(saleID) })
	}
	if cfg.LateSurge {
		go e.loop(ctx, "late_surge", 100*time.Millisecond, func() { e.doJoin(saleID, "late") })
	}
	if cfg.RefreshStorm || cfg.MixedRealistic {
		go e.loop(ctx, "refresh_storm", 30*time.Millisecond, func() { e.doRefresh(saleID) })
	}
	if cfg.ReconnectChurn || cfg.MixedRealistic {
		go e.loop(ctx, "reconnect_churn", 500*time.Millisecond, func() { e.doReconnect(saleID) })
	}
	if cfg.ScalperBots || cfg.MixedRealistic {
		go e.loop(ctx, "scalper_bots", 50*time.Millisecond, func() { e.doScalper(saleID) })
	}
}

func anyOn(c Config) bool {
	return c.LaunchSpike || c.BackdoorRace || c.LateSurge || c.RefreshStorm ||
		c.ReconnectChurn || c.ScalperBots || c.MixedRealistic
}

func (e *Engine) loop(ctx context.Context, name string, interval time.Duration, fn func()) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			fn()
			e.perScn[name].Add(1)
		}
	}
}

func (e *Engine) record(statusCode int, latency time.Duration, blocked bool) {
	atomic.AddInt64(&e.stats.RequestsTotal, 1)
	if statusCode >= 500 {
		atomic.AddInt64(&e.stats.ErrorsTotal, 1)
	}
	if blocked {
		atomic.AddInt64(&e.stats.BlockedTotal, 1)
	}
	atomic.StoreInt64(&e.stats.LastLatencyMs, latency.Milliseconds())
}

func (e *Engine) registerFake(tag string) (token string) {
	email := fmt.Sprintf("sim_%s_%d@load.test", tag, time.Now().UnixNano())
	body, _ := json.Marshal(map[string]string{"email": email, "password": "testpass123"})
	start := time.Now()
	res, err := e.client.Post(e.baseURL+"/v1/auth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		e.record(0, time.Since(start), false)
		return ""
	}
	defer res.Body.Close()
	e.record(res.StatusCode, time.Since(start), res.StatusCode == 429)
	if res.StatusCode != 201 && res.StatusCode != 200 {
		return ""
	}
	var out struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(res.Body).Decode(&out)
	return out.Token
}

func (e *Engine) doLaunchSpike(saleID string) {
	tok := e.registerFake("spike")
	if tok == "" {
		return
	}
	start := time.Now()
	req, _ := http.NewRequest(http.MethodPost, e.baseURL+"/v1/sales/"+saleID+"/waiting-room/join", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	res, err := e.client.Do(req)
	if err != nil {
		e.record(0, time.Since(start), false)
		return
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	e.record(res.StatusCode, time.Since(start), res.StatusCode == 429 || res.StatusCode == 503)
}

func (e *Engine) doBackdoor(saleID string) {
	tok := e.registerFake("back")
	start := time.Now()
	// Checkout without admission token
	req, _ := http.NewRequest(http.MethodPost, e.baseURL+"/v1/sales/"+saleID+"/checkout",
		bytes.NewReader([]byte(`{"hold_id":"fake","idempotency_key":"x"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	res, err := e.client.Do(req)
	if err != nil {
		return
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	e.record(res.StatusCode, time.Since(start), res.StatusCode == 403 || res.StatusCode == 429)
}

func (e *Engine) doJoin(saleID, tag string) {
	tok := e.registerFake(tag)
	if tok == "" {
		return
	}
	start := time.Now()
	req, _ := http.NewRequest(http.MethodPost, e.baseURL+"/v1/sales/"+saleID+"/waiting-room/join", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	res, _ := e.client.Do(req)
	if res != nil {
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		e.record(res.StatusCode, time.Since(start), false)
	}
}

func (e *Engine) doRefresh(saleID string) {
	tok := e.registerFake("refresh")
	if tok == "" {
		return
	}
	_ = e.doJoinOnce(saleID, tok)
	start := time.Now()
	req, _ := http.NewRequest(http.MethodGet, e.baseURL+"/v1/sales/"+saleID+"/waiting-room/status", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	res, _ := e.client.Do(req)
	if res != nil {
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		e.record(res.StatusCode, time.Since(start), false)
	}
}

func (e *Engine) doReconnect(saleID string) {
	tok := e.registerFake("reconn")
	if tok == "" {
		return
	}
	_ = e.doJoinOnce(saleID, tok)
	time.Sleep(100 * time.Millisecond)
	start := time.Now()
	req, _ := http.NewRequest(http.MethodGet, e.baseURL+"/v1/sales/"+saleID+"/waiting-room/status", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	res, _ := e.client.Do(req)
	if res != nil {
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		e.record(res.StatusCode, time.Since(start), false)
	}
}

func (e *Engine) doJoinOnce(saleID, tok string) error {
	req, _ := http.NewRequest(http.MethodPost, e.baseURL+"/v1/sales/"+saleID+"/waiting-room/join", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	res, err := e.client.Do(req)
	if res != nil {
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
	}
	return err
}

func (e *Engine) doScalper(saleID string) {
	tok := e.registerFake("scalper")
	start := time.Now()
	req, _ := http.NewRequest(http.MethodPost, e.baseURL+"/v1/tickets/transfer", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	res, err := e.client.Do(req)
	if err != nil {
		return
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	e.record(res.StatusCode, time.Since(start), res.StatusCode == 403)
	// rapid checkout attempts
	req2, _ := http.NewRequest(http.MethodPost, e.baseURL+"/v1/sales/"+saleID+"/checkout",
		bytes.NewReader([]byte(`{"hold_id":"x","idempotency_key":"s"}`)))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+tok)
	res2, _ := e.client.Do(req2)
	if res2 != nil {
		io.Copy(io.Discard, res2.Body)
		res2.Body.Close()
		e.record(res2.StatusCode, time.Since(start), res2.StatusCode == 403 || res2.StatusCode == 429)
	}
}

// SaleParams configures a demo sale for load tests.
type SaleParams struct {
	TotalSeats       int
	AdmitPerMinute   int
	WaitingRoomCap   int
}

func defaultSaleParams() SaleParams {
	return SaleParams{TotalSeats: 500, AdmitPerMinute: 6000, WaitingRoomCap: 100000}
}

// SetupDemo creates event + open sale via admin API.
func (e *Engine) SetupDemo(ctx context.Context) (saleID string, err error) {
	return e.SetupDemoWithParams(ctx, defaultSaleParams())
}

// SetupDemoWithParams creates a fresh open sale with the given capacity settings.
func (e *Engine) SetupDemoWithParams(ctx context.Context, p SaleParams) (saleID string, err error) {
	if p.TotalSeats <= 0 {
		p.TotalSeats = 500
	}
	if p.AdmitPerMinute <= 0 {
		p.AdmitPerMinute = 6000
	}
	if p.WaitingRoomCap <= 0 {
		p.WaitingRoomCap = 100000
	}

	evBody, _ := json.Marshal(map[string]string{
		"name": "Live Demo Concert", "venue": "Arena", "starts_at": "2026-12-01T20:00:00Z",
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/v1/admin/events", bytes.NewReader(evBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Key", e.adminKey)
	res, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var ev struct {
		EventID string `json:"event_id"`
	}
	_ = json.NewDecoder(res.Body).Decode(&ev)

	opens := time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339)
	ends := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	saleBody, _ := json.Marshal(map[string]interface{}{
		"event_id": ev.EventID, "opens_at": opens, "ends_at": ends,
		"total_seats": p.TotalSeats, "admit_per_minute": p.AdmitPerMinute, "waiting_room_cap": p.WaitingRoomCap,
	})
	req2, _ := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/v1/admin/sales", bytes.NewReader(saleBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Admin-Key", e.adminKey)
	res2, err := e.client.Do(req2)
	if err != nil {
		return "", err
	}
	defer res2.Body.Close()
	var sale struct {
		SaleID string `json:"sale_id"`
	}
	_ = json.NewDecoder(res2.Body).Decode(&sale)
	e.SetSaleID(sale.SaleID)
	return sale.SaleID, nil
}
