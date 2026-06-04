package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/arkin/traffic/internal/simulation"
)

func (s *Server) getSimulation(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, s.sim.GetConfig())
}

func (s *Server) setSimulation(w http.ResponseWriter, r *http.Request) {
	var cfg simulation.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	saleID := s.sim.SaleID()
	if saleID == "" {
		id, err := s.sim.SetupDemo(r.Context())
		if err != nil {
			Error(w, http.StatusInternalServerError, "setup_failed", err.Error())
			return
		}
		saleID = id
	}
	s.sim.Update(cfg)
	snap := s.sim.StatsSnapshot()
	JSON(w, http.StatusOK, map[string]interface{}{
		"config":  cfg,
		"sale_id": saleID,
		"active":  snap.ActiveScenarios,
	})
}

func (s *Server) demoSetup(w http.ResponseWriter, r *http.Request) {
	id, err := s.sim.SetupDemo(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "setup_failed", err.Error())
		return
	}
	s.sim.SetSaleID(id)
	JSON(w, http.StatusOK, map[string]string{"sale_id": id})
}

func (s *Server) liveDashboard(w http.ResponseWriter, r *http.Request) {
	saleID := r.URL.Query().Get("sale_id")
	if saleID == "" {
		saleID = s.sim.SaleID()
	}
	if saleID == "" {
		Error(w, http.StatusBadRequest, "no_sale", "run demo setup first")
		return
	}
	st, err := s.dash.Live(r.Context(), saleID, s.sim)
	if err != nil {
		Error(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	JSON(w, http.StatusOK, st)
}
