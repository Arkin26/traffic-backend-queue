package loadtest

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestServiceCapsAndQueue(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "jobs"), filepath.Join(dir, "results"))
	limits := Limits{
		MaxVUs:        100,
		MaxQueueDepth: 2,
		MaxConcurrent: 1,
		MaxSteady:     30 * time.Second,
		DefaultVUs:    50,
	}
	svc := NewServiceWithLimits(store, limits)

	req := RunRequest{Scenario: "launch_spike", VUsMax: 999, Steady: "5m"}
	job, err := svc.Submit(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if job.VUsMax != 100 {
		t.Fatalf("expected capped VUs 100, got %d", job.VUsMax)
	}
	if job.Steady != "30s" {
		t.Fatalf("expected capped steady 30s, got %s", job.Steady)
	}

	now := time.Now().UTC()
	if err := store.WriteResult(Result{
		RunID:     job.RunID,
		Status:    StatusRunning,
		StartedAt: &now,
	}); err != nil {
		t.Fatal(err)
	}

	_, err = svc.Submit(context.Background(), RunRequest{Scenario: "queue_fairness"})
	if err != ErrAlreadyBusy {
		t.Fatalf("expected busy, got %v", err)
	}
}
