package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/arkin/traffic/internal/loadtest"
	"github.com/arkin/traffic/internal/simulation"
)

func (s *Server) demoConfig(w http.ResponseWriter, r *http.Request) {
	lim := s.loadtest.Limits()
	JSON(w, http.StatusOK, map[string]interface{}{
		"demo_key_required": s.cfg.DemoAPIKey != "",
		"max_vus":           lim.MaxVUs,
		"max_queue":         lim.MaxQueueDepth,
		"max_concurrent":    lim.MaxConcurrent,
		"max_steady":        lim.MaxSteady.String(),
		"default_vus":       lim.DefaultVUs,
	})
}

func (s *Server) startLoadTest(w http.ResponseWriter, r *http.Request) {
	var req loadtest.RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	if !loadtest.ValidScenario(req.Scenario) {
		Error(w, http.StatusBadRequest, "invalid_scenario", "unknown scenario")
		return
	}

	saleID := req.SaleID
	if saleID == "" {
		saleID = s.sim.SaleID()
	}
	if saleID == "" {
		id, err := s.sim.SetupDemoWithParams(r.Context(), simulation.SaleParams{
			TotalSeats:     req.TotalSeats,
			AdmitPerMinute: req.AdmitPerMinute,
			WaitingRoomCap: req.WaitingRoomCap,
		})
		if err != nil {
			Error(w, http.StatusInternalServerError, "setup_failed", err.Error())
			return
		}
		saleID = id
	}
	req.SaleID = saleID

	job, err := s.loadtest.Submit(r.Context(), req)
	if errors.Is(err, loadtest.ErrQueueFull) || errors.Is(err, loadtest.ErrAlreadyBusy) {
		Error(w, http.StatusTooManyRequests, "loadtest_busy", err.Error())
		return
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "submit_failed", err.Error())
		return
	}

	JSON(w, http.StatusAccepted, map[string]interface{}{
		"run_id":   job.RunID,
		"sale_id":  saleID,
		"scenario": job.Scenario,
		"status":   loadtest.StatusQueued,
	})
}

func (s *Server) getLoadTest(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runID")
	res, err := s.loadtest.GetRun(runID)
	if loadtest.IsNotExist(err) {
		Error(w, http.StatusNotFound, "not_found", "run not found")
		return
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	JSON(w, http.StatusOK, res)
}

func (s *Server) listLoadTests(w http.ResponseWriter, r *http.Request) {
	runs, err := s.loadtest.ListRuns(20)
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if runs == nil {
		runs = []loadtest.Result{}
	}
	JSON(w, http.StatusOK, map[string]interface{}{
		"runs":       runs,
		"scenarios":  loadtest.AllowedScenarios,
	})
}

func (s *Server) demoReset(w http.ResponseWriter, r *http.Request) {
	var req loadtest.ResetRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	id, err := s.sim.SetupDemoWithParams(r.Context(), simulation.SaleParams{
		TotalSeats:     req.TotalSeats,
		AdmitPerMinute: req.AdmitPerMinute,
		WaitingRoomCap: req.WaitingRoomCap,
	})
	if err != nil {
		Error(w, http.StatusInternalServerError, "reset_failed", err.Error())
		return
	}
	JSON(w, http.StatusOK, map[string]string{
		"sale_id": id,
		"message": "fresh demo sale ready",
	})
}
