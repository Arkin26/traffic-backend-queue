package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/redis/go-redis/v9"

	"github.com/arkin/traffic/internal/antibot"
	"github.com/arkin/traffic/internal/auth"
	"github.com/arkin/traffic/internal/config"
	"github.com/arkin/traffic/internal/dashboard"
	"github.com/arkin/traffic/internal/inventory"
	"github.com/arkin/traffic/internal/loadtest"
	"github.com/arkin/traffic/internal/metrics"
	"github.com/arkin/traffic/internal/orders"
	"github.com/arkin/traffic/internal/simulation"
	"github.com/arkin/traffic/internal/waitingroom"
)

type Server struct {
	cfg    config.Config
	db     *pgxpool.Pool
	rdb    *redis.Client
	queue  *waitingroom.Service
	inv    *inventory.Service
	ord    *orders.Service
	limit  *antibot.Limiter
	sim      *simulation.Engine
	dash     *dashboard.Service
	loadtest *loadtest.Service
}

func NewServer(
	cfg config.Config,
	db *pgxpool.Pool,
	rdb *redis.Client,
	queue *waitingroom.Service,
	inv *inventory.Service,
	ord *orders.Service,
	limit *antibot.Limiter,
	sim *simulation.Engine,
	dash *dashboard.Service,
	lt *loadtest.Service,
) *Server {
	return &Server{cfg: cfg, db: db, rdb: rdb, queue: queue, inv: inv, ord: ord, limit: limit, sim: sim, dash: dash, loadtest: lt}
}

func (s *Server) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/health/ready", s.ready)
	r.Handle("/metrics", metrics.Handler())

	r.Post("/v1/auth/register", s.register)
	r.Post("/v1/auth/login", s.login)

	r.Route("/v1/demo", func(r chi.Router) {
		r.Use(s.demoAuth)
		r.Get("/config", s.demoConfig)
		r.Post("/setup", s.demoSetup)
		r.Post("/reset", s.demoReset)
		r.Put("/simulation", s.setSimulation)
		r.Get("/simulation", s.getSimulation)
		r.Get("/dashboard", s.liveDashboard)
		r.Post("/loadtest", s.startLoadTest)
		r.Get("/loadtests", s.listLoadTests)
		r.Get("/loadtest/{runID}", s.getLoadTest)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(s.cfg.JWTSecret))
		r.Post("/v1/sales/{saleID}/waiting-room/join", s.queueJoin)
		r.Get("/v1/sales/{saleID}/waiting-room/status", s.queueStatus)
		r.Get("/v1/sales/{saleID}/waiting-room/stream", s.queueStream)
		r.Get("/v1/orders/{orderID}", s.getOrder)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.AdmissionMiddleware(s.cfg.JWTSecret))
		r.Get("/v1/sales/{saleID}/seats", s.listSeats)
		r.Post("/v1/sales/{saleID}/holds", s.createHold)
		r.Post("/v1/sales/{saleID}/checkout", s.checkout)
	})

	r.Route("/v1/admin", func(r chi.Router) {
		r.Use(s.adminAuth)
		r.Post("/events", s.createEvent)
		r.Post("/sales", s.createSale)
		r.Get("/sales/{saleID}/stats", s.saleStats)
		r.Get("/sales/{saleID}/queue-entries", s.queueEntries)
	})

	r.Post("/v1/tickets/transfer", func(w http.ResponseWriter, r *http.Request) {
		_ = s.ord.Transfer(r.Context())
		Error(w, http.StatusForbidden, "transfer_blocked", "Tickets are non-transferable")
	})

	fileServer := staticHandler()
	r.Get("/", serveIndex)
	r.Get("/index.html", serveIndex)
	r.Handle("/styles.css", fileServer)
	r.Handle("/app.js", fileServer)

	return r
}

func (s *Server) adminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Admin-Key") != s.cfg.AdminAPIKey {
			Error(w, http.StatusUnauthorized, "unauthorized", "invalid admin key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) demoAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.DemoAPIKey == "" {
			next.ServeHTTP(w, r)
			return
		}
		key := r.Header.Get("X-Demo-Key")
		if key == "" {
			key = r.URL.Query().Get("demo_key")
		}
		if key != s.cfg.DemoAPIKey {
			Error(w, http.StatusUnauthorized, "unauthorized", "invalid or missing demo key (X-Demo-Key header)")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Ping(r.Context()); err != nil {
		Error(w, http.StatusServiceUnavailable, "not_ready", "database unavailable")
		return
	}
	JSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func clientIP(r *http.Request) string {
	if x := r.Header.Get("X-Forwarded-For"); x != "" {
		return x
	}
	return r.RemoteAddr
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal", "hash failed")
		return
	}
	var id string
	err = s.db.QueryRow(r.Context(), `
		INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
		req.Email, hash).Scan(&id)
	if err != nil {
		Error(w, http.StatusConflict, "exists", "email already registered")
		return
	}
	token, _ := auth.IssueUserToken(s.cfg.JWTSecret, id, 24*time.Hour)
	JSON(w, http.StatusCreated, map[string]string{"user_id": id, "token": token})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	var id, hash string
	err := s.db.QueryRow(r.Context(), `
		SELECT id, password_hash FROM users WHERE email = $1`, req.Email).Scan(&id, &hash)
	if err != nil || auth.CheckPassword(hash, req.Password) != nil {
		Error(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		return
	}
	token, _ := auth.IssueUserToken(s.cfg.JWTSecret, id, 24*time.Hour)
	JSON(w, http.StatusOK, map[string]string{"user_id": id, "token": token})
}

func (s *Server) createEvent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Venue    string `json:"venue"`
		StartsAt string `json:"starts_at"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	starts, _ := time.Parse(time.RFC3339, req.StartsAt)
	var id string
	_ = s.db.QueryRow(r.Context(), `
		INSERT INTO events (name, venue, starts_at) VALUES ($1, $2, $3) RETURNING id`,
		req.Name, req.Venue, starts).Scan(&id)
	JSON(w, http.StatusCreated, map[string]string{"event_id": id})
}

func (s *Server) createSale(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EventID          string `json:"event_id"`
		OpensAt          string `json:"opens_at"`
		EndsAt           string `json:"ends_at"`
		TotalSeats       int    `json:"total_seats"`
		AdmitPerMinute   int    `json:"admit_per_minute"`
		WaitingRoomCap   int    `json:"waiting_room_cap"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	opens, _ := time.Parse(time.RFC3339, req.OpensAt)
	ends, _ := time.Parse(time.RFC3339, req.EndsAt)
	if req.TotalSeats <= 0 {
		req.TotalSeats = 2000
	}
	if req.AdmitPerMinute <= 0 {
		req.AdmitPerMinute = 5000
	}
	if req.WaitingRoomCap <= 0 {
		req.WaitingRoomCap = 100000
	}
	var id string
	err := s.db.QueryRow(r.Context(), `
		INSERT INTO sales (event_id, phase, opens_at, ends_at, total_seats, admit_per_minute, waiting_room_cap)
		VALUES ($1, 'general_sale', $2, $3, $4, $5, $6) RETURNING id`,
		req.EventID, opens, ends, req.TotalSeats, req.AdmitPerMinute, req.WaitingRoomCap).Scan(&id)
	if err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	_ = s.inv.CreateSeats(r.Context(), id, req.TotalSeats)
	JSON(w, http.StatusCreated, map[string]string{"sale_id": id})
}

func (s *Server) queueJoin(w http.ResponseWriter, r *http.Request) {
	saleID := chi.URLParam(r, "saleID")
	userID := auth.UserIDFromContext(r.Context())
	ip := clientIP(r)

	ok, err := s.limit.AllowJoin(r.Context(), userID, ip)
	if err != nil || !ok {
		Error(w, http.StatusTooManyRequests, "rate_limited", "too many join attempts")
		return
	}

	pos, isNew, err := s.queue.Join(r.Context(), saleID, userID)
	if errors.Is(err, waitingroom.ErrSaleNotOpen) {
		Error(w, http.StatusForbidden, "sale_closed", "sale is not open for queue joining")
		return
	}
	if errors.Is(err, waitingroom.ErrWaitingRoomFull) {
		Error(w, http.StatusServiceUnavailable, "room_full", "waiting room at capacity")
		return
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	status := http.StatusOK
	if isNew {
		status = http.StatusCreated
	}
	JSON(w, status, map[string]interface{}{
		"position": pos,
		"message":  "Refreshing does not change your place in line.",
	})
}

func (s *Server) queueStatus(w http.ResponseWriter, r *http.Request) {
	saleID := chi.URLParam(r, "saleID")
	userID := auth.UserIDFromContext(r.Context())
	st, err := s.queue.Status(r.Context(), saleID, userID)
	if errors.Is(err, waitingroom.ErrNotInQueue) {
		Error(w, http.StatusNotFound, "not_in_queue", "join the waiting room first")
		return
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	JSON(w, http.StatusOK, st)
}

func (s *Server) queueStream(w http.ResponseWriter, r *http.Request) {
	saleID := chi.URLParam(r, "saleID")
	userID := auth.UserIDFromContext(r.Context())
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, ok := w.(http.Flusher)
	if !ok {
		Error(w, http.StatusInternalServerError, "internal", "streaming not supported")
		return
	}
	for i := 0; i < 5; i++ {
		st, err := s.queue.Status(r.Context(), saleID, userID)
		if err != nil {
			break
		}
		b, _ := json.Marshal(st)
		_, _ = w.Write([]byte("data: " + string(b) + "\n\n"))
		flusher.Flush()
		if st.Admitted {
			break
		}
		time.Sleep(time.Duration(st.RetryAfterMs) * time.Millisecond)
	}
}

func (s *Server) listSeats(w http.ResponseWriter, r *http.Request) {
	saleID := chi.URLParam(r, "saleID")
	seats, err := s.inv.ListAvailable(r.Context(), saleID, 200)
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	JSON(w, http.StatusOK, map[string]interface{}{"seats": seats})
}

func (s *Server) createHold(w http.ResponseWriter, r *http.Request) {
	saleID := chi.URLParam(r, "saleID")
	userID := auth.UserIDFromContext(r.Context())
	var req struct {
		SeatIDs        []string `json:"seat_ids"`
		IdempotencyKey string   `json:"idempotency_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = "default"
	}
	hold, err := s.inv.Hold(r.Context(), saleID, userID, req.IdempotencyKey, req.SeatIDs)
	if errors.Is(err, inventory.ErrSeatUnavailable) {
		Error(w, http.StatusConflict, "unavailable", "seat not available")
		return
	}
	if err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	JSON(w, http.StatusOK, hold)
}

func (s *Server) checkout(w http.ResponseWriter, r *http.Request) {
	saleID := chi.URLParam(r, "saleID")
	userID := auth.UserIDFromContext(r.Context())
	jti, _ := r.Context().Value("jti").(string)
	ok, err := auth.ConsumeAdmission(r.Context(), s.rdb, saleID, jti, s.cfg.AdmissionTTL*2)
	if err != nil || !ok {
		Error(w, http.StatusForbidden, "admission_used", "admission token already used — request a new one from queue status")
		return
	}
	ip := clientIP(r)
	ok, _ = s.limit.AllowCheckout(r.Context(), userID, ip)
	if !ok {
		Error(w, http.StatusTooManyRequests, "rate_limited", "checkout rate limit exceeded")
		return
	}
	var req struct {
		HoldID         string `json:"hold_id"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = "default"
	}
	res, err := s.ord.Checkout(r.Context(), saleID, userID, req.IdempotencyKey, req.HoldID)
	if errors.Is(err, orders.ErrTicketCapReached) {
		Error(w, http.StatusForbidden, "cap_exceeded", "ticket purchase limit reached")
		return
	}
	if err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	JSON(w, http.StatusOK, res)
}

func (s *Server) getOrder(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	userID := auth.UserIDFromContext(r.Context())
	var status string
	err := s.db.QueryRow(r.Context(), `
		SELECT status FROM orders WHERE id = $1 AND user_id = $2`, orderID, userID).Scan(&status)
	if err != nil {
		Error(w, http.StatusNotFound, "not_found", "order not found")
		return
	}
	JSON(w, http.StatusOK, map[string]string{"order_id": orderID, "status": status})
}

func (s *Server) saleStats(w http.ResponseWriter, r *http.Request) {
	saleID := chi.URLParam(r, "saleID")
	st, _ := s.ord.Stats(r.Context(), saleID)
	JSON(w, http.StatusOK, st)
}

func (s *Server) queueEntries(w http.ResponseWriter, r *http.Request) {
	saleID := chi.URLParam(r, "saleID")
	entries, err := s.queue.QueueEntriesForAssert(r.Context(), saleID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	JSON(w, http.StatusOK, map[string]interface{}{"entries": entries})
}
