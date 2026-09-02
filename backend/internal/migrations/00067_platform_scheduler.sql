-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00067_platform_scheduler
-- Add tables for the generic scheduler registry and runs
-- ============================================================

CREATE TABLE platform_scheduled_jobs (
    job_name TEXT PRIMARY KEY,
    cron_expr TEXT NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    last_status TEXT, -- 'success', 'error', 'running'
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE platform_job_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_name TEXT NOT NULL REFERENCES platform_scheduled_jobs(job_name) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    status TEXT NOT NULL, -- 'running', 'success', 'error'
    error_message TEXT,
    items_processed INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_platform_jobs_next_run ON platform_scheduled_jobs(next_run_at) WHERE is_enabled = TRUE;
CREATE INDEX idx_platform_job_runs_job_name ON platform_job_runs(job_name, started_at DESC);

-- Seed permissions for the scheduler
INSERT INTO permissions (key, resource, action, description) VALUES
    ('platform.scheduler.view', 'platform.scheduler', 'view', 'Allows viewing scheduled jobs and their execution history'),
    ('platform.scheduler.manage', 'platform.scheduler', 'manage', 'Allows manually triggering jobs and enabling/disabling them')
ON CONFLICT (key) DO UPDATE SET 
    resource = EXCLUDED.resource,
    action = EXCLUDED.action,
    description = EXCLUDED.description;

-- Grant permissions to Owner and Admin roles
UPDATE roles 
SET permissions = array_cat(permissions, ARRAY['platform.scheduler.view', 'platform.scheduler.manage'])
WHERE name IN ('owner', 'admin')
AND NOT ('platform.scheduler.view' = ANY(permissions));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

UPDATE roles 
SET permissions = array_remove(array_remove(permissions, 'platform.scheduler.view'), 'platform.scheduler.manage')
WHERE name IN ('owner', 'admin');
DELETE FROM permissions WHERE key IN ('platform.scheduler.view', 'platform.scheduler.manage');

DROP TABLE IF EXISTS platform_job_runs;
DROP TABLE IF EXISTS platform_scheduled_jobs;

-- +goose StatementEnd
