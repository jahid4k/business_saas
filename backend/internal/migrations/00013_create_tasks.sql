-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00013_create_tasks
-- Creates the org-scoped tasks table and seeds tasks.* permissions,
-- granting them to system roles with the same distribution as projects.*.
-- ============================================================

CREATE TABLE tasks (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT        NOT NULL UNIQUE DEFAULT ('task_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),

    org_id          UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    title           TEXT        NOT NULL,
    description     TEXT        NOT NULL DEFAULT '',
    status          TEXT        NOT NULL DEFAULT 'todo'
                                  CHECK (status IN ('todo', 'in_progress', 'done', 'cancelled')),

    due_date        TIMESTAMPTZ,

    created_by      UUID        REFERENCES users(id) ON DELETE SET NULL,
    assigned_to     UUID        REFERENCES users(id) ON DELETE SET NULL,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_tasks_public_id    ON tasks (public_id);
CREATE INDEX idx_tasks_org_id              ON tasks (org_id);
CREATE INDEX idx_tasks_org_status          ON tasks (org_id, status);
CREATE INDEX idx_tasks_org_assigned_to     ON tasks (org_id, assigned_to) WHERE assigned_to IS NOT NULL;
CREATE INDEX idx_tasks_org_due_date        ON tasks (org_id, due_date)    WHERE due_date IS NOT NULL;
CREATE INDEX idx_tasks_created_at          ON tasks (created_at);

COMMENT ON TABLE  tasks             IS 'Org-scoped tasks (reference module for permission and tenant-isolation enforcement)';
COMMENT ON COLUMN tasks.public_id   IS 'Public API-facing task id, generated with task_ prefix';
COMMENT ON COLUMN tasks.status      IS 'Lifecycle status: todo, in_progress, done, cancelled';
COMMENT ON COLUMN tasks.due_date    IS 'Optional deadline for the task';
COMMENT ON COLUMN tasks.created_by  IS 'User who created the task; NULL if that user has since been removed';
COMMENT ON COLUMN tasks.assigned_to IS 'User the task is assigned to; NULL if unassigned or assignee removed';

-- ----------------------------------------------------------
-- Permissions: tasks.view / tasks.create / tasks.update / tasks.delete
-- Distribution mirrors projects.* exactly.
-- ----------------------------------------------------------

INSERT INTO permissions (key, resource, action, description) VALUES
    ('tasks.view',   'tasks', 'view',   'View tasks'),
    ('tasks.create', 'tasks', 'create', 'Create tasks'),
    ('tasks.update', 'tasks', 'update', 'Update tasks'),
    ('tasks.delete', 'tasks', 'delete', 'Delete tasks')
ON CONFLICT (key) DO NOTHING;

-- owner, admin: all four
UPDATE roles
SET permissions = (
    SELECT ARRAY(
        SELECT DISTINCT p FROM UNNEST(permissions || ARRAY[
            'tasks.view', 'tasks.create', 'tasks.update', 'tasks.delete'
        ]::TEXT[]) AS p ORDER BY p
    )
), updated_at = NOW()
WHERE org_id IS NULL AND name IN ('owner', 'admin');

-- manager, member: view/create/update, no delete
UPDATE roles
SET permissions = (
    SELECT ARRAY(
        SELECT DISTINCT p FROM UNNEST(permissions || ARRAY[
            'tasks.view', 'tasks.create', 'tasks.update'
        ]::TEXT[]) AS p ORDER BY p
    )
), updated_at = NOW()
WHERE org_id IS NULL AND name IN ('manager', 'member');

-- viewer: view only
UPDATE roles
SET permissions = (
    SELECT ARRAY(
        SELECT DISTINCT p FROM UNNEST(permissions || ARRAY[
            'tasks.view'
        ]::TEXT[]) AS p ORDER BY p
    )
), updated_at = NOW()
WHERE org_id IS NULL AND name = 'viewer';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(SELECT p FROM UNNEST(permissions) p WHERE p NOT LIKE 'tasks.%'),
    updated_at = NOW()
WHERE org_id IS NULL;

DELETE FROM permissions WHERE resource = 'tasks';

DROP TABLE IF EXISTS tasks;

-- +goose StatementEnd