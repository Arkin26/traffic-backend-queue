package loadtest

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreJobAndResult(t *testing.T) {
	dir := t.TempDir()
	jobs := filepath.Join(dir, "jobs")
	results := filepath.Join(dir, "results")
	store := NewStore(jobs, results)

	job := Job{
		RunID:    "test-run-1",
		Scenario: "launch_spike",
		VUsMax:   10,
		RampUp:   "5s",
		Steady:   "10s",
		RampDown: "5s",
		CreatedAt: time.Now().UTC(),
	}
	if err := store.WriteJob(job); err != nil {
		t.Fatal(err)
	}

	pending, err := store.ListPendingJobs()
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending: %v err=%v", pending, err)
	}

	now := time.Now().UTC()
	res := Result{
		RunID:      job.RunID,
		Scenario:   job.Scenario,
		Status:     StatusPassed,
		Passed:     true,
		FinishedAt: &now,
	}
	if err := store.WriteResult(res); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetStatus(job.RunID)
	if err != nil || !got.Passed {
		t.Fatalf("status: %+v err=%v", got, err)
	}

	pending2, _ := store.ListPendingJobs()
	if len(pending2) != 0 {
		t.Fatalf("expected no pending after result, got %d", len(pending2))
	}
}

func TestParseK6Report(t *testing.T) {
	raw := []byte(`{"run_id":"r1","scenario":"launch_spike","passed":true,"failures":[],"metrics":{"http_reqs":100}}`)
	res, err := ParseK6Report("r1", raw)
	if err != nil || !res.Passed {
		t.Fatalf("parse: %+v err=%v", res, err)
	}
}

func TestValidScenario(t *testing.T) {
	if !ValidScenario("launch_spike") {
		t.Fatal("expected valid")
	}
	if ValidScenario("not_a_scenario") {
		t.Fatal("expected invalid")
	}
}

func TestDefaultDirs(t *testing.T) {
	os.Unsetenv("LOADTEST_JOBS_DIR")
	os.Unsetenv("LOADTEST_RESULTS_DIR")
	j, r := DefaultDirs()
	if j != "/loadtest/jobs" || r != "/loadtest/results" {
		t.Fatalf("defaults: %s %s", j, r)
	}
}
