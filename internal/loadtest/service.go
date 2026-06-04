package loadtest

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	store   *Store
	limits  Limits
}

func NewService(store *Store) *Service {
	return &Service{store: store, limits: DefaultLimits()}
}

func NewServiceWithLimits(store *Store, limits Limits) *Service {
	return &Service{store: store, limits: limits}
}

func NewDefaultService() *Service {
	jobs, results := DefaultDirs()
	return NewService(NewStore(jobs, results))
}

func (s *Service) Limits() Limits {
	return s.limits
}

func (s *Service) Submit(ctx context.Context, req RunRequest) (*Job, error) {
	if !ValidScenario(req.Scenario) {
		return nil, ErrInvalidScenario(req.Scenario)
	}
	pending, running, err := s.store.CountActive()
	if err != nil {
		return nil, err
	}
	if err := s.limits.CanAccept(pending, running); err != nil {
		return nil, err
	}
	s.limits.Apply(&req)

	job := Job{
		RunID:          uuid.New().String(),
		Scenario:       req.Scenario,
		SaleID:         req.SaleID,
		VUsMax:         req.VUsMax,
		RampUp:         req.RampUp,
		Steady:         req.Steady,
		RampDown:       req.RampDown,
		AdmitPerMinute: req.AdmitPerMinute,
		WaitingRoomCap: req.WaitingRoomCap,
		TotalSeats:     req.TotalSeats,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.store.WriteJob(job); err != nil {
		return nil, fmt.Errorf("write job: %w", err)
	}
	return &job, nil
}

func (s *Service) GetRun(runID string) (*Result, error) {
	return s.store.GetStatus(runID)
}

func (s *Service) ListRuns(limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.store.ListResults(limit)
}

func (s *Service) Store() *Store {
	return s.store
}
