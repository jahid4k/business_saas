package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

type Service interface {
	// Register defines a scheduled job in memory and syncs its cron definition to the DB.
	Register(jobName, cronExpr string, fn JobFunc)
	
	// Start kicks off the background ticker. Should be run in a goroutine.
	Start(ctx context.Context)

	// Trigger allows manual execution of a job via the admin interface.
	Trigger(ctx context.Context, jobName string) error

	// Admin API helpers
	ListJobs(ctx context.Context) (*JobListResponse, error)
	ListJobRuns(ctx context.Context, jobName string, limit, offset int) (*JobRunListResponse, error)
}

type serviceImpl struct {
	repo       Repository
	redis      *redis.Client
	registry   map[string]JobFunc
	cronParser cron.Parser
	mu         sync.RWMutex
}

func NewService(repo Repository, redisClient *redis.Client) Service {
	return &serviceImpl{
		repo:       repo,
		redis:      redisClient,
		registry:   make(map[string]JobFunc),
		cronParser: cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
	}
}

func (s *serviceImpl) Register(jobName, cronExpr string, fn JobFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.registry[jobName] = fn

	// Ensure job exists in DB with the registered cron
	// NextRunAt is initialized by parsing the cron
	var nextRun *time.Time
	sched, err := s.cronParser.Parse(cronExpr)
	if err == nil {
		t := sched.Next(time.Now())
		nextRun = &t
	} else {
		slog.Error("scheduler: failed to parse cron expression during registration", "job", jobName, "cron", cronExpr, "error", err)
	}

	job := &ScheduledJob{
		JobName:   jobName,
		CronExpr:  cronExpr,
		IsEnabled: true,
		NextRunAt: nextRun,
	}

	// Called during startup DI wiring, before any request context exists.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.repo.UpsertJob(ctx, job); err != nil {
		slog.Error("scheduler: failed to upsert job registration", "job", jobName, "error", err)
	} else {
		slog.Info("scheduler: job registered", "job", jobName, "cron", cronExpr)
	}
}

func (s *serviceImpl) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	slog.Info("scheduler: background ticker started")

	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler: context done, stopping ticker")
			return
		case <-ticker.C:
			s.runPendingJobs(ctx)
		}
	}
}

func (s *serviceImpl) runPendingJobs(ctx context.Context) {
	// 1. Fetch pending jobs (next_run_at <= NOW)
	// We use a short timeout context to prevent stalling the ticker
	qCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	jobs, err := s.repo.FindPendingJobs(qCtx)
	if err != nil {
		slog.Error("scheduler: failed to find pending jobs", "error", err)
		return
	}

	// 2. Attempt to run each job
	for _, job := range jobs {
		// Spawn in goroutine to allow concurrent jobs
		go func(jobName string) {
			_ = s.executeJob(ctx, jobName, "scheduled")
		}(job.JobName)
	}
}

func (s *serviceImpl) Trigger(ctx context.Context, jobName string) error {
	s.mu.RLock()
	_, exists := s.registry[jobName]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("scheduler: job %q not registered in memory", jobName)
	}

	go func() {
		_ = s.executeJob(context.Background(), jobName, "manual")
	}()

	return nil
}

func (s *serviceImpl) executeJob(ctx context.Context, jobName string, runType string) error {
	// 1. Try to acquire redis lock
	lockKey := fmt.Sprintf("scheduler:lock:%s", jobName)
	// Lock for 15 minutes max to prevent zombie locks if process dies
	acquired, err := s.redis.SetNX(ctx, lockKey, runType, 15*time.Minute).Result()
	if err != nil {
		slog.Error("scheduler: redis lock error", "job", jobName, "error", err)
		return err
	}
	if !acquired {
		// Someone else is running this job
		return nil
	}
	// Ensure we release the lock when done
	defer s.redis.Del(context.Background(), lockKey)

	// 2. Look up the function in memory
	s.mu.RLock()
	fn, exists := s.registry[jobName]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("job func %s not found in registry", jobName)
	}

	// 3. Create run record
	run := &JobRun{
		JobName:   jobName,
		StartedAt: time.Now(),
		Status:    "running",
	}
	if err := s.repo.CreateJobRun(ctx, run); err != nil {
		slog.Error("scheduler: failed to create run record", "job", jobName, "error", err)
		return err
	}

	// 4. Update job status to running
	job, err := s.repo.GetJob(ctx, jobName)
	if err != nil || job == nil {
		return fmt.Errorf("could not get job metadata")
	}

	// 5. Execute
	slog.Info("scheduler: executing job", "job", jobName, "type", runType)
	
	items, err := fn(ctx)
	
	finishedAt := time.Now()
	run.FinishedAt = &finishedAt
	run.ItemsProcessed = items

	var nextRun *time.Time
	var newStatus string
	var consecFailures = job.ConsecutiveFailures

	if err != nil {
		slog.Error("scheduler: job failed", "job", jobName, "error", err)
		run.Status = "error"
		errMsg := err.Error()
		run.ErrorMessage = &errMsg
		newStatus = "error"
		consecFailures++
	} else {
		slog.Info("scheduler: job succeeded", "job", jobName, "items", items)
		run.Status = "success"
		newStatus = "success"
		consecFailures = 0
	}

	// 6. Calculate next run (only if scheduled run, manual runs shouldn't advance the schedule if it hasn't passed)
	sched, parseErr := s.cronParser.Parse(job.CronExpr)
	if parseErr == nil {
		t := sched.Next(time.Now())
		nextRun = &t
	}

	// 7. Save results
	if updErr := s.repo.UpdateJobRun(ctx, run); updErr != nil {
		slog.Error("scheduler: failed to update run record", "job", jobName, "error", updErr)
	}

	if updErr := s.repo.UpdateJobStatus(ctx, jobName, newStatus, nextRun, consecFailures); updErr != nil {
		slog.Error("scheduler: failed to update job status", "job", jobName, "error", updErr)
	}

	return err
}

func (s *serviceImpl) ListJobs(ctx context.Context) (*JobListResponse, error) {
	jobs, err := s.repo.FindAllJobs(ctx)
	if err != nil {
		return nil, err
	}
	return &JobListResponse{Jobs: jobs, Total: len(jobs)}, nil
}

func (s *serviceImpl) ListJobRuns(ctx context.Context, jobName string, limit, offset int) (*JobRunListResponse, error) {
	runs, total, err := s.repo.FindJobRuns(ctx, jobName, limit, offset)
	if err != nil {
		return nil, err
	}
	return &JobRunListResponse{Runs: runs, Total: total}, nil
}
