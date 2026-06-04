package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/arkin/traffic/internal/loadtest"
)

func main() {
	jobsDir, resultsDir := loadtest.DefaultDirs()
	scriptsDir := os.Getenv("K6_SCRIPTS_DIR")
	if scriptsDir == "" {
		scriptsDir = "/scripts"
	}
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://api:8080"
	}
	adminKey := os.Getenv("ADMIN_KEY")
	if adminKey == "" {
		adminKey = "dev-admin-key"
	}

	store := loadtest.NewStore(jobsDir, resultsDir)
	if err := store.EnsureDirs(); err != nil {
		log.Fatalf("dirs: %v", err)
	}
	if n, err := store.CleanStaleRunning(2 * time.Hour); err != nil {
		log.Printf("clean stale: %v", err)
	} else if n > 0 {
		log.Printf("removed %d stale running result(s)", n)
	}
	log.Printf("loadtest-runner watching %s (results %s)", jobsDir, resultsDir)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		jobs, err := store.ListPendingJobs()
		if err != nil {
			log.Printf("list jobs: %v", err)
			continue
		}
		busy, err := store.HasRunning()
		if err != nil {
			log.Printf("running check: %v", err)
			continue
		}
		if busy {
			continue
		}
		if len(jobs) == 0 {
			continue
		}
		job := jobs[0]
		if err := runJob(store, scriptsDir, baseURL, adminKey, job); err != nil {
			log.Printf("run %s: %v", job.RunID, err)
		}
	}
}

func runJob(store *loadtest.Store, scriptsDir, baseURL, adminKey string, job loadtest.Job) error {
	if err := store.WriteRunning(job.RunID); err != nil {
		return err
	}

	script := filepath.Join(scriptsDir, "scenarios", job.Scenario+".js")
	if _, err := os.Stat(script); err != nil {
		now := time.Now().UTC()
		if werr := store.WriteResult(loadtest.Result{
			RunID:      job.RunID,
			Scenario:   job.Scenario,
			Status:     loadtest.StatusError,
			Passed:     false,
			Error:      "scenario script not found: " + script,
			FinishedAt: &now,
		}); werr != nil {
			return werr
		}
		_ = os.Remove(filepath.Join(store.JobsDir, job.RunID+".json"))
		return nil
	}

	summaryPath := filepath.Join(store.ResultsDir, job.RunID+".json")
	env := os.Environ()
	env = append(env,
		"K6_SCENARIO="+job.Scenario,
		"K6_RUN_ID="+job.RunID,
		"RUN_ID="+job.RunID,
		"K6_RESULTS_DIR="+store.ResultsDir,
		"BASE_URL="+baseURL,
		"ADMIN_KEY="+adminKey,
		"K6_VUS_MAX="+strconv.Itoa(job.VUsMax),
		"K6_RAMP_UP="+job.RampUp,
		"K6_STEADY="+job.Steady,
		"K6_RAMP_DOWN="+job.RampDown,
		"K6_ADMIT_PER_MIN="+strconv.Itoa(job.AdmitPerMinute),
		"K6_WAITING_ROOM_CAP="+strconv.Itoa(job.WaitingRoomCap),
		"K6_TOTAL_SEATS="+strconv.Itoa(job.TotalSeats),
	)
	if job.SaleID != "" {
		env = append(env, "SALE_ID="+job.SaleID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "k6", "run", script)
	cmd.Env = env
	cmd.Dir = scriptsDir
	out, err := cmd.CombinedOutput()
	log.Printf("k6 run %s: %v\n%s", job.RunID, err, truncate(string(out), 2000))

	raw, readErr := os.ReadFile(summaryPath)
	if readErr != nil {
		now := time.Now().UTC()
		msg := "k6 finished without summary"
		if err != nil {
			msg = err.Error() + ": " + truncate(string(out), 500)
		}
		if werr := store.WriteResult(loadtest.Result{
			RunID:      job.RunID,
			Scenario:   job.Scenario,
			SaleID:     job.SaleID,
			Status:     loadtest.StatusError,
			Passed:     false,
			Error:      msg,
			FinishedAt: &now,
		}); werr != nil {
			return werr
		}
		_ = os.Remove(filepath.Join(store.JobsDir, job.RunID+".json"))
		return nil
	}

	res, parseErr := loadtest.ParseK6Report(job.RunID, raw)
	if parseErr != nil {
		// summary file may be raw k6 format; try wrapping
		var generic map[string]any
		if json.Unmarshal(raw, &generic) == nil {
			now := time.Now().UTC()
			passed := err == nil
			status := loadtest.StatusPassed
			if !passed {
				status = loadtest.StatusFailed
			}
			return store.WriteResult(loadtest.Result{
				RunID:      job.RunID,
				Scenario:   job.Scenario,
				SaleID:     job.SaleID,
				Status:     status,
				Passed:     passed,
				Extra:      generic,
				FinishedAt: &now,
				Error:      errString(err),
			})
		}
		now := time.Now().UTC()
		return store.WriteResult(loadtest.Result{
			RunID:      job.RunID,
			Scenario:   job.Scenario,
			Status:     loadtest.StatusError,
			Passed:     false,
			Error:      parseErr.Error(),
			FinishedAt: &now,
		})
	}
	if res.SaleID == "" {
		res.SaleID = job.SaleID
	}
	if res.Scenario == "" {
		res.Scenario = job.Scenario
	}
	if err != nil && res.Status == loadtest.StatusPassed {
		res.Status = loadtest.StatusFailed
		res.Passed = false
		if res.Failures == nil {
			res.Failures = []string{err.Error()}
		}
	}
	if err := store.WriteResult(*res); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(store.JobsDir, job.RunID+".json"))
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
