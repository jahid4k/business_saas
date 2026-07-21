-- +goose Up
-- +goose StatementBegin

-- Add related context columns to tasks
ALTER TABLE tasks 
ADD COLUMN IF NOT EXISTS related_type TEXT,
ADD COLUMN IF NOT EXISTS related_id TEXT;

CREATE INDEX IF NOT EXISTS idx_tasks_org_related ON tasks (org_id, related_type, related_id) WHERE related_type IS NOT NULL;

-- Add tasks.view_all permission
INSERT INTO permissions (key, resource, action, description) VALUES
    ('tasks.view_all', 'tasks', 'view_all', 'View all tasks in the organization')
ON CONFLICT (key) DO NOTHING;

-- Grant tasks.view_all to owner and admin roles
UPDATE roles
SET permissions = (
    SELECT ARRAY(
        SELECT DISTINCT p FROM UNNEST(permissions || ARRAY['tasks.view_all']::TEXT[]) AS p ORDER BY p
    )
), updated_at = NOW()
WHERE org_id IS NULL AND name IN ('owner', 'admin');

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(SELECT p FROM UNNEST(permissions) p WHERE p != 'tasks.view_all'),
    updated_at = NOW()
WHERE org_id IS NULL AND name IN ('owner', 'admin');

DELETE FROM permissions WHERE key = 'tasks.view_all';

DROP INDEX IF EXISTS idx_tasks_org_related;

ALTER TABLE tasks 
DROP COLUMN IF EXISTS related_type,
DROP COLUMN IF EXISTS related_id;

-- +goose StatementEnd
