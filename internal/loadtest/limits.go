package loadtest

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrQueueFull    = errors.New("load test queue is full; wait for the current run to finish")
	ErrAlreadyBusy  = errors.New("a load test is already running")
)

// Limits caps shared-demo load tests so the API stays responsive.
type Limits struct {
	MaxVUs           int
	MaxQueueDepth    int
	MaxConcurrent    int
	MaxSteady        time.Duration
	DefaultVUs       int
}

func DefaultLimits() Limits {
	return Limits{
		MaxVUs:        150,
		MaxQueueDepth: 3,
		MaxConcurrent: 1,
		MaxSteady:     90 * time.Second,
		DefaultVUs:    80,
	}
}

func (l Limits) Apply(req *RunRequest) {
	if req.VUsMax <= 0 {
		req.VUsMax = l.DefaultVUs
	}
	if l.MaxVUs > 0 && req.VUsMax > l.MaxVUs {
		req.VUsMax = l.MaxVUs
	}
	if req.RampUp == "" {
		req.RampUp = "10s"
	}
	if req.Steady == "" {
		req.Steady = "30s"
	}
	if req.RampDown == "" {
		req.RampDown = "10s"
	}
	if l.MaxSteady > 0 {
		if d, err := time.ParseDuration(req.Steady); err == nil && d > l.MaxSteady {
			req.Steady = l.MaxSteady.String()
		}
	}
	if req.AdmitPerMinute <= 0 {
		req.AdmitPerMinute = 6000
	}
	if req.WaitingRoomCap <= 0 {
		req.WaitingRoomCap = 100000
	}
	if req.TotalSeats <= 0 {
		req.TotalSeats = 500
	}
}

func (l Limits) CanAccept(pending int, running int) error {
	maxQ := l.MaxQueueDepth
	if maxQ <= 0 {
		maxQ = 3
	}
	maxRun := l.MaxConcurrent
	if maxRun <= 0 {
		maxRun = 1
	}
	if running >= maxRun {
		return ErrAlreadyBusy
	}
	if pending+running >= maxQ {
		return ErrQueueFull
	}
	return nil
}

func ParseSteady(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid steady duration %q: %w", s, err)
	}
	return d, nil
}
