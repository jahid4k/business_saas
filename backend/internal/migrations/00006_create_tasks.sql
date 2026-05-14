-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00006_create_tasks
-- Creates the tasks table — the Phase 1 permission test module.
--
-- Every query against this table MUST include a business_id
-- WHERE clause. Tenant isolation is enforced at the application
-- layer (repository), backed by the composite index below.
-- ============================================================

-- Task status as a Postgres enum — matches TaskStatus in task/model.go
CREATE TYPE task_status AS ENUM ('todo', 'in_progress', 'done');

CREATE TABLE tasks (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id UUID        NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    title       TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    status      task_status NOT NULL DEFAULT 'todo',
    created_by  UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Primary list query: all tasks for a business, newest first
-- This is also the tenant isolation index — every query uses business_id
CREATE INDEX idx_tasks_business_id_created_at
    ON tasks (business_id, created_at DESC);

-- Filter by status within a business
CREATE INDEX idx_tasks_business_id_status
    ON tasks (business_id, status);

-- Lookup tasks created by a specific user within a business
CREATE INDEX idx_tasks_business_id_created_by
    ON tasks (business_id, created_by);

COMMENT ON TABLE  tasks            IS 'Phase 1 test module — permission-gated CRUD, scoped per business';
COMMENT ON COLUMN tasks.business_id IS 'Tenant key — every query must include this in the WHERE clause';
COMMENT ON COLUMN tasks.created_by  IS 'User who created the task — cannot be deleted while tasks exist';
COMMENT ON COLUMN tasks.status      IS 'todo | in_progress | done';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS tasks;
DROP TYPE IF EXISTS task_status;

-- +goose StatementEnd