package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	UpsertJob(ctx context.Context, job *ScheduledJob) error
	FindAllJobs(ctx context.Context) ([]*ScheduledJob, error)
	GetJob(ctx context.Context, jobName string) (*ScheduledJob, error)
	FindPendingJobs(ctx context.Context) ([]*ScheduledJob, error)
	UpdateJobStatus(ctx context.Context, jobName, status string, nextRunAt *time.Time, consecutiveFailures int) error
	CreateJobRun(ctx context.Context, run *JobRun) error
	UpdateJobRun(ctx context.Context, run *JobRun) error
	FindJobRuns(ctx context.Context, jobName string, limit int, offset int) ([]*JobRun, int, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

func (r *repoImpl) UpsertJob(ctx context.Context, job *ScheduledJob) error {
	q := `INSERT INTO platform_scheduled_jobs (job_name, cron_expr, is_enabled, next_run_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (job_name) DO UPDATE SET 
		cron_expr = EXCLUDED.cron_expr,
		updated_at = NOW()
		RETURNING is_enabled, last_run_at, next_run_at, last_status, consecutive_failures, created_at, updated_at`

	err := r.db.QueryRow(ctx, q, job.JobName, job.CronExpr, job.IsEnabled, job.NextRunAt).Scan(
		&job.IsEnabled, &job.LastRunAt, &job.NextRunAt, &job.LastStatus,
		&job.ConsecutiveFailures, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("scheduler: UpsertJob: %w", err)
	}
	return nil
}

func (r *repoImpl) FindAllJobs(ctx context.Context) ([]*ScheduledJob, error) {
	q := `SELECT job_name, cron_expr, is_enabled, last_run_at, next_run_at, last_status, consecutive_failures, created_at, updated_at
		FROM platform_scheduled_jobs ORDER BY job_name`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("scheduler: FindAllJobs: %w", err)
	}
	defer rows.Close()

	var jobs []*ScheduledJob
	for rows.Next() {
		job := &ScheduledJob{}
		if err := rows.Scan(&job.JobName, &job.CronExpr, &job.IsEnabled, &job.LastRunAt, &job.NextRunAt, &job.LastStatus, &job.ConsecutiveFailures, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scheduler: FindAllJobs scan: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (r *repoImpl) GetJob(ctx context.Context, jobName string) (*ScheduledJob, error) {
	q := `SELECT job_name, cron_expr, is_enabled, last_run_at, next_run_at, last_status, consecutive_failures, created_at, updated_at
		FROM platform_scheduled_jobs WHERE job_name = $1`

	job := &ScheduledJob{}
	err := r.db.QueryRow(ctx, q, jobName).Scan(
		&job.JobName, &job.CronExpr, &job.IsEnabled, &job.LastRunAt, &job.NextRunAt,
		&job.LastStatus, &job.ConsecutiveFailures, &job.CreatedAt, &job.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scheduler: GetJob: %w", err)
	}
	return job, nil
}

func (r *repoImpl) FindPendingJobs(ctx context.Context) ([]*ScheduledJob, error) {
	q := `SELECT job_name, cron_expr, is_enabled, last_run_at, next_run_at, last_status, consecutive_failures, created_at, updated_at
		FROM platform_scheduled_jobs 
		WHERE is_enabled = TRUE AND (next_run_at IS NULL OR next_run_at <= NOW())`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("scheduler: FindPendingJobs: %w", err)
	}
	defer rows.Close()

	var jobs []*ScheduledJob
	for rows.Next() {
		job := &ScheduledJob{}
		if err := rows.Scan(&job.JobName, &job.CronExpr, &job.IsEnabled, &job.LastRunAt, &job.NextRunAt, &job.LastStatus, &job.ConsecutiveFailures, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scheduler: FindPendingJobs scan: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (r *repoImpl) UpdateJobStatus(ctx context.Context, jobName, status string, nextRunAt *time.Time, consecutiveFailures int) error {
	q := `UPDATE platform_scheduled_jobs SET 
		last_status = $1, 
		next_run_at = $2, 
		last_run_at = NOW(),
		consecutive_failures = $3,
		updated_at = NOW()
		WHERE job_name = $4`
	_, err := r.db.Exec(ctx, q, status, nextRunAt, consecutiveFailures, jobName)
	if err != nil {
		return fmt.Errorf("scheduler: UpdateJobStatus: %w", err)
	}
	return nil
}

func (r *repoImpl) CreateJobRun(ctx context.Context, run *JobRun) error {
	q := `INSERT INTO platform_job_runs (job_name, started_at, status, error_message, items_processed)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`
	return r.db.QueryRow(ctx, q, run.JobName, run.StartedAt, run.Status, run.ErrorMessage, run.ItemsProcessed).Scan(&run.ID)
}

func (r *repoImpl) UpdateJobRun(ctx context.Context, run *JobRun) error {
	q := `UPDATE platform_job_runs SET finished_at = $1, status = $2, error_message = $3, items_processed = $4 WHERE id = $5`
	_, err := r.db.Exec(ctx, q, run.FinishedAt, run.Status, run.ErrorMessage, run.ItemsProcessed, run.ID)
	if err != nil {
		return fmt.Errorf("scheduler: UpdateJobRun: %w", err)
	}
	return nil
}

func (r *repoImpl) FindJobRuns(ctx context.Context, jobName string, limit int, offset int) ([]*JobRun, int, error) {
	countQ := `SELECT COUNT(*) FROM platform_job_runs WHERE job_name = $1`
	var total int
	if err := r.db.QueryRow(ctx, countQ, jobName).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("scheduler: FindJobRuns count: %w", err)
	}

	q := `SELECT id, job_name, started_at, finished_at, status, error_message, items_processed
		FROM platform_job_runs WHERE job_name = $1 ORDER BY started_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, jobName, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("scheduler: FindJobRuns: %w", err)
	}
	defer rows.Close()

	var runs []*JobRun
	for rows.Next() {
		run := &JobRun{}
		if err := rows.Scan(&run.ID, &run.JobName, &run.StartedAt, &run.FinishedAt, &run.Status, &run.ErrorMessage, &run.ItemsProcessed); err != nil {
			return nil, 0, fmt.Errorf("scheduler: FindJobRuns scan: %w", err)
		}
		runs = append(runs, run)
	}
	return runs, total, nil
}
