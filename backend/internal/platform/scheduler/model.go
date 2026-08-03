package scheduler

import (
	"context"
	"time"
)

// SystemUserID is the sentinel "system" user (seeded in migration 00069) used as the
// actor for scheduler-triggered writes that need a valid created_by/actor FK but have
// no human requester. It is never a real login (no email/password_hash).
const SystemUserID = "00000000-0000-0000-0000-000000000001"

type ScheduledJob struct {
	JobName             string     `json:"job_name" db:"job_name"`
	CronExpr            string     `json:"cron_expr" db:"cron_expr"`
	IsEnabled           bool       `json:"is_enabled" db:"is_enabled"`
	LastRunAt           *time.Time `json:"last_run_at,omitempty" db:"last_run_at"`
	NextRunAt           *time.Time `json:"next_run_at,omitempty" db:"next_run_at"`
	LastStatus          *string    `json:"last_status,omitempty" db:"last_status"`
	ConsecutiveFailures int        `json:"consecutive_failures" db:"consecutive_failures"`
	CreatedAt           time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at" db:"updated_at"`
}

type JobRun struct {
	ID             string     `json:"id" db:"id"`
	JobName        string     `json:"job_name" db:"job_name"`
	StartedAt      time.Time  `json:"started_at" db:"started_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty" db:"finished_at"`
	Status         string     `json:"status" db:"status"`
	ErrorMessage   *string    `json:"error_message,omitempty" db:"error_message"`
	ItemsProcessed int        `json:"items_processed" db:"items_processed"`
}

type JobFunc func(ctx context.Context) (itemsProcessed int, err error)

type JobListResponse struct {
	Jobs  []*ScheduledJob `json:"jobs"`
	Total int             `json:"total"`
}

type JobRunListResponse struct {
	Runs  []*JobRun `json:"runs"`
	Total int       `json:"total"`
}
