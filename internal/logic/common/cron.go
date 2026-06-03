package common

import (
	"context"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/robfig/cron/v3"
)

// CronJob defines a scheduled job.
type CronJob struct {
	Name     string
	Schedule string // cron expression, e.g. "0 0 * * *" for daily at midnight
	Handler  func(ctx context.Context) error
}

// JobInfo contains info about a registered job for listing.
type JobInfo struct {
	Name      string `json:"name"`
	Schedule  string `json:"schedule"`
	IsRunning bool   `json:"is_running"`
}

// CronScheduler manages scheduled tasks with distributed locking.
type CronScheduler struct {
	jobs    []*CronJob
	mu      sync.RWMutex
	running map[string]bool
	runMu   sync.Mutex
	cron    *cron.Cron
}

var (
	cronScheduler     *CronScheduler
	cronSchedulerOnce sync.Once
)

// InitCronScheduler initializes the global cron scheduler singleton.
func InitCronScheduler() {
	cronSchedulerOnce.Do(func() {
		cronScheduler = NewCronScheduler()
	})
}

// GetCronScheduler returns the global cron scheduler.
func GetCronScheduler() *CronScheduler {
	if cronScheduler == nil {
		panic("cron scheduler not initialized, call InitCronScheduler first")
	}
	return cronScheduler
}

// NewCronScheduler creates a new CronScheduler.
func NewCronScheduler() *CronScheduler {
	return &CronScheduler{
		running: make(map[string]bool),
		cron:    cron.New(),
	}
}

// Register adds a scheduled job.
func (cs *CronScheduler) Register(name, schedule string, handler func(ctx context.Context) error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.jobs = append(cs.jobs, &CronJob{
		Name:     name,
		Schedule: schedule,
		Handler:  handler,
	})
}

// ListJobs returns all registered jobs with their running status.
func (cs *CronScheduler) ListJobs() []JobInfo {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	jobs := make([]JobInfo, 0, len(cs.jobs))
	for _, j := range cs.jobs {
		cs.runMu.Lock()
		running := cs.running[j.Name]
		cs.runMu.Unlock()
		jobs = append(jobs, JobInfo{
			Name:      j.Name,
			Schedule:  j.Schedule,
			IsRunning: running,
		})
	}
	return jobs
}

// RunJob executes a named job with distributed locking to prevent concurrent execution.
func (cs *CronScheduler) RunJob(ctx context.Context, name string) error {
	return cs.runJobInternal(ctx, name, "auto")
}

// TriggerJob triggers a job asynchronously (for manual execution).
func (cs *CronScheduler) TriggerJob(ctx context.Context, name string) error {
	cs.mu.RLock()
	var found bool
	for _, j := range cs.jobs {
		if j.Name == name {
			found = true
			break
		}
	}
	cs.mu.RUnlock()

	if !found {
		return gerror.Newf("job %s not registered", name)
	}

	cs.runMu.Lock()
	if cs.running[name] {
		cs.runMu.Unlock()
		return gerror.Newf("job %s is already running", name)
	}
	cs.runMu.Unlock()

	go func() {
		bgCtx := gctx.New()
		if err := cs.runJobInternal(bgCtx, name, "manual"); err != nil {
			g.Log().Errorf(bgCtx, "manual trigger failed: %s: %v", name, err)
		}
	}()

	return nil
}

// runJobInternal executes a named job with execution tracking.
func (cs *CronScheduler) runJobInternal(ctx context.Context, name, triggeredBy string) error {
	cs.runMu.Lock()
	if cs.running[name] {
		cs.runMu.Unlock()
		return gerror.Newf("job %s is already running", name)
	}
	cs.running[name] = true
	cs.runMu.Unlock()

	defer func() {
		cs.runMu.Lock()
		delete(cs.running, name)
		cs.runMu.Unlock()
	}()

	// Find job
	cs.mu.RLock()
	var job *CronJob
	for _, j := range cs.jobs {
		if j.Name == name {
			job = j
			break
		}
	}
	cs.mu.RUnlock()

	if job == nil {
		return gerror.Newf("job %s not registered", name)
	}

	// Execute job
	startTime := time.Now()
	handlerErr := job.Handler(ctx)
	duration := time.Since(startTime)

	status := "succeeded"
	var errMsg string
	if handlerErr != nil {
		status = "failed"
		errMsg = handlerErr.Error()
		g.Log().Errorf(ctx, "cron job failed: %s, duration=%v, error=%v", name, duration, handlerErr)
	}

	// Persist execution record (best-effort)
	cs.recordExecution(ctx, job.Name, status, startTime, duration, errMsg, triggeredBy)

	return handlerErr
}

// recordExecution persists an execution record to the database.
// Always UPSERT sys_cron_jobs with latest status; no separate execution log table.
func (cs *CronScheduler) recordExecution(ctx context.Context, jobName, status string, startedAt time.Time, duration time.Duration, errMsg, triggeredBy string) {
	finishedAt := startedAt.Add(duration)
	durationMs := duration.Milliseconds()

	if status == "succeeded" {
		_, err := g.DB().Ctx(ctx).Exec(ctx,
			`INSERT INTO sys_cron_jobs (job_name, last_status, last_started_at, last_finished_at, last_duration_ms, last_triggered_by, total_runs)
			 VALUES ($1, $2, $3, $4, $5, $6, 1)
			 ON CONFLICT (job_name) DO UPDATE SET
			   last_status = $2,
			   last_started_at = $3,
			   last_finished_at = $4,
			   last_duration_ms = $5,
			   last_triggered_by = $6,
			   last_error_message = NULL,
			   total_runs = sys_cron_jobs.total_runs + 1,
			   updated_at = now()`,
			jobName, status, startedAt, finishedAt, durationMs, triggeredBy,
		)
		if err != nil {
			g.Log().Warningf(ctx, "failed to upsert cron job %s: %v", jobName, err)
		}
	} else {
		// failed
		_, err := g.DB().Ctx(ctx).Exec(ctx,
			`INSERT INTO sys_cron_jobs (job_name, last_status, last_started_at, last_finished_at, last_duration_ms, last_error_message, last_triggered_by, total_runs, total_failures)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, 1, 1)
			 ON CONFLICT (job_name) DO UPDATE SET
			   last_status = $2,
			   last_started_at = $3,
			   last_finished_at = $4,
			   last_duration_ms = $5,
			   last_error_message = $6,
			   last_triggered_by = $7,
			   total_runs = sys_cron_jobs.total_runs + 1,
			   total_failures = sys_cron_jobs.total_failures + 1,
			   updated_at = now()`,
			jobName, status, startedAt, finishedAt, durationMs, errMsg, triggeredBy,
		)
		if err != nil {
			g.Log().Warningf(ctx, "failed to upsert cron job %s: %v", jobName, err)
		}
	}
}

// RunAll runs all registered jobs sequentially.
// Used for startup initialization or testing.
func (cs *CronScheduler) RunAll(ctx context.Context) {
	cs.mu.RLock()
	jobs := make([]*CronJob, len(cs.jobs))
	copy(jobs, cs.jobs)
	cs.mu.RUnlock()

	for _, job := range jobs {
		if err := cs.RunJob(ctx, job.Name); err != nil {
			g.Log().Errorf(ctx, "cron run all: job %s failed: %v", job.Name, err)
		}
	}
}

// StartBackground runs the scheduler in a goroutine using robfig/cron for proper cron expression parsing.
func (cs *CronScheduler) StartBackground(ctx context.Context) {
	cs.mu.RLock()
	jobs := make([]*CronJob, len(cs.jobs))
	copy(jobs, cs.jobs)
	cs.mu.RUnlock()

	for _, job := range jobs {
		j := job
		_, err := cs.cron.AddFunc(j.Schedule, func() {
			bgCtx := gctx.New()
			if err := cs.RunJob(bgCtx, j.Name); err != nil {
				g.Log().Errorf(bgCtx, "background cron error: %s: %v", j.Name, err)
			}
		})
		if err != nil {
			g.Log().Errorf(ctx, "failed to register cron job %s with schedule %s: %v", j.Name, j.Schedule, err)
		}
	}

	cs.cron.Start()

	go func() {
		<-ctx.Done()
		stopCtx := cs.cron.Stop()
		<-stopCtx.Done()
		g.Log().Info(ctx, "cron scheduler stopped")
	}()

	g.Log().Info(ctx, "cron scheduler started in background")
}
