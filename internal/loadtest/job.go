package loadtest

import "time"

type Status string

const (
	StatusQueued   Status = "queued"
	StatusRunning  Status = "running"
	StatusPassed   Status = "passed"
	StatusFailed   Status = "failed"
	StatusError    Status = "error"
)

// Job is written to the shared jobs volume for loadtest-runner.
type Job struct {
	RunID            string    `json:"run_id"`
	Scenario         string    `json:"scenario"`
	SaleID           string    `json:"sale_id,omitempty"`
	VUsMax           int       `json:"vus_max"`
	RampUp           string    `json:"ramp_up"`
	Steady           string    `json:"steady"`
	RampDown         string    `json:"ramp_down"`
	AdmitPerMinute   int       `json:"admit_per_minute"`
	WaitingRoomCap   int       `json:"waiting_room_cap"`
	TotalSeats       int       `json:"total_seats"`
	CreatedAt        time.Time `json:"created_at"`
}

// Result is the evaluated k6 summary written by runner or API.
type Result struct {
	RunID      string            `json:"run_id"`
	Scenario   string            `json:"scenario"`
	SaleID     string            `json:"sale_id,omitempty"`
	Status     Status            `json:"status"`
	Passed     bool              `json:"passed"`
	Failures   []string          `json:"failures,omitempty"`
	Metrics    map[string]any    `json:"metrics,omitempty"`
	Extra      map[string]any    `json:"extra,omitempty"`
	StartedAt  *time.Time        `json:"started_at,omitempty"`
	FinishedAt *time.Time        `json:"finished_at,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// RunRequest is the API body for POST /v1/demo/loadtest.
type RunRequest struct {
	Scenario       string `json:"scenario"`
	SaleID         string `json:"sale_id,omitempty"`
	VUsMax         int    `json:"vus_max"`
	RampUp         string `json:"ramp_up"`
	Steady         string `json:"steady"`
	RampDown       string `json:"ramp_down"`
	AdmitPerMinute int    `json:"admit_per_minute"`
	WaitingRoomCap int    `json:"waiting_room_cap"`
	TotalSeats     int    `json:"total_seats"`
}

// ResetRequest configures a fresh demo sale.
type ResetRequest struct {
	AdmitPerMinute int `json:"admit_per_minute"`
	WaitingRoomCap int `json:"waiting_room_cap"`
	TotalSeats     int `json:"total_seats"`
}

var AllowedScenarios = []string{
	"launch_spike",
	"backdoor_race",
	"late_surge",
	"refresh_storm",
	"reconnect_churn",
	"bot_checkout",
	"scalper_network",
	"queue_fairness",
	"second_sale_regression",
	"mixed_realistic",
}

func ValidScenario(name string) bool {
	for _, s := range AllowedScenarios {
		if s == name {
			return true
		}
	}
	return false
}
