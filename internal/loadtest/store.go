package loadtest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Store struct {
	JobsDir    string
	ResultsDir string
}

func NewStore(jobsDir, resultsDir string) *Store {
	return &Store{JobsDir: jobsDir, ResultsDir: resultsDir}
}

func (s *Store) EnsureDirs() error {
	if err := os.MkdirAll(s.JobsDir, 0o777); err != nil {
		return err
	}
	return os.MkdirAll(s.ResultsDir, 0o777)
}

func (s *Store) WriteJob(job Job) error {
	if err := s.EnsureDirs(); err != nil {
		return err
	}
	path := filepath.Join(s.JobsDir, job.RunID+".json")
	b, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o666); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) ReadJob(runID string) (*Job, error) {
	path := filepath.Join(s.JobsDir, runID+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var job Job
	if err := json.Unmarshal(b, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// CountActive returns queued jobs plus runs marked running (not yet finished).
func (s *Store) CountActive() (pending int, running int, err error) {
	if err = s.EnsureDirs(); err != nil {
		return 0, 0, err
	}
	jobs, err := s.ListPendingJobs()
	if err != nil {
		return 0, 0, err
	}
	pending = len(jobs)
	entries, err := os.ReadDir(s.ResultsDir)
	if err != nil {
		return pending, 0, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		runID := strings.TrimSuffix(e.Name(), ".json")
		res, rerr := s.ReadResult(runID)
		if rerr != nil {
			continue
		}
		if res.Status == StatusRunning {
			if res.StartedAt != nil && time.Since(*res.StartedAt) > 45*time.Minute {
				continue
			}
			running++
		}
	}
	return pending, running, nil
}

func (s *Store) HasRunning() (bool, error) {
	_, running, err := s.CountActive()
	return running > 0, err
}

func (s *Store) ListPendingJobs() ([]Job, error) {
	if err := s.EnsureDirs(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.JobsDir)
	if err != nil {
		return nil, err
	}
	var jobs []Job
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasSuffix(e.Name(), ".tmp") {
			continue
		}
		runID := strings.TrimSuffix(e.Name(), ".json")
		if s.isJobClaimed(runID) {
			continue
		}
		job, err := s.ReadJob(runID)
		if err != nil {
			continue
		}
		jobs = append(jobs, *job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.Before(jobs[j].CreatedAt)
	})
	return jobs, nil
}

func (s *Store) isJobClaimed(runID string) bool {
	res, err := s.ReadResult(runID)
	if err != nil {
		return false
	}
	return res.Status == StatusRunning || res.Status == StatusPassed ||
		res.Status == StatusFailed || res.Status == StatusError
}

func (s *Store) HasResult(runID string) bool {
	_, err := os.Stat(filepath.Join(s.ResultsDir, runID+".json"))
	return err == nil
}

func (s *Store) WriteResult(res Result) error {
	if err := s.EnsureDirs(); err != nil {
		return err
	}
	path := filepath.Join(s.ResultsDir, res.RunID+".json")
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o666); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) ReadResult(runID string) (*Result, error) {
	path := filepath.Join(s.ResultsDir, runID+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var res Result
	if err := json.Unmarshal(b, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *Store) WriteRunning(runID string) error {
	now := time.Now().UTC()
	return s.WriteResult(Result{
		RunID:     runID,
		Status:    StatusRunning,
		StartedAt: &now,
	})
}

func (s *Store) GetStatus(runID string) (*Result, error) {
	if res, err := s.ReadResult(runID); err == nil {
		if res.Status == StatusRunning && res.StartedAt != nil {
			if time.Since(*res.StartedAt) > 45*time.Minute {
				res.Status = StatusError
				res.Error = "run timed out (stale running state)"
			}
		}
		return res, nil
	}
	if _, err := s.ReadJob(runID); err == nil {
		return &Result{RunID: runID, Status: StatusQueued}, nil
	}
	return nil, os.ErrNotExist
}

// CleanStaleRunning removes abandoned "running" markers so jobs can be retried.
func (s *Store) CleanStaleRunning(maxAge time.Duration) (int, error) {
	entries, err := os.ReadDir(s.ResultsDir)
	if err != nil {
		return 0, err
	}
	removed := 0
	now := time.Now().UTC()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		runID := strings.TrimSuffix(e.Name(), ".json")
		res, err := s.ReadResult(runID)
		if err != nil || res.Status != StatusRunning {
			continue
		}
		if res.StartedAt != nil && now.Sub(*res.StartedAt) < maxAge {
			continue
		}
		if res.FinishedAt != nil {
			continue
		}
		_ = os.Remove(filepath.Join(s.ResultsDir, runID+".json"))
		removed++
	}
	return removed, nil
}

func (s *Store) ListResults(limit int) ([]Result, error) {
	if err := s.EnsureDirs(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.ResultsDir)
	if err != nil {
		return nil, err
	}
	var results []Result
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		runID := strings.TrimSuffix(e.Name(), ".json")
		res, err := s.ReadResult(runID)
		if err != nil {
			continue
		}
		results = append(results, *res)
	}
	sort.Slice(results, func(i, j int) bool {
		ti := results[i].FinishedAt
		tj := results[j].FinishedAt
		if ti == nil && tj == nil {
			return results[i].RunID > results[j].RunID
		}
		if ti == nil {
			return true
		}
		if tj == nil {
			return false
		}
		return ti.After(*tj)
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// ParseK6Report maps k6 summary export JSON into Result.
func ParseK6Report(runID string, raw []byte) (*Result, error) {
	var report struct {
		RunID      string         `json:"run_id"`
		Scenario   string         `json:"scenario"`
		SaleID     string         `json:"sale_id"`
		Passed     bool           `json:"passed"`
		Failures   []string       `json:"failures"`
		Metrics    map[string]any `json:"metrics"`
		Extra      map[string]any `json:"extra"`
		FinishedAt string         `json:"finished_at"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, err
	}
	status := StatusPassed
	if !report.Passed {
		status = StatusFailed
	}
	var finished *time.Time
	if report.FinishedAt != "" {
		t, err := time.Parse(time.RFC3339, report.FinishedAt)
		if err == nil {
			finished = &t
		}
	}
	return &Result{
		RunID:      runID,
		Scenario:   report.Scenario,
		SaleID:     report.SaleID,
		Status:     status,
		Passed:     report.Passed,
		Failures:   report.Failures,
		Metrics:    report.Metrics,
		Extra:      report.Extra,
		FinishedAt: finished,
	}, nil
}

func IsNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist) || os.IsNotExist(err)
}

func DefaultDirs() (jobs, results string) {
	jobs = os.Getenv("LOADTEST_JOBS_DIR")
	results = os.Getenv("LOADTEST_RESULTS_DIR")
	if jobs == "" {
		jobs = "/loadtest/jobs"
	}
	if results == "" {
		results = "/loadtest/results"
	}
	return jobs, results
}

func ErrInvalidScenario(name string) error {
	return fmt.Errorf("unknown scenario: %s", name)
}
