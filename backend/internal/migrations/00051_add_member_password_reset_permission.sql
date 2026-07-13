-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00051_add_member_password_reset_permission
-- Adds a permission for an admin to reset another member's
-- password directly. Kept separate from members.update since
-- this is materially more sensitive than editing title/role/status.
-- Depends on: 00003_create_permissions, 00004_create_roles
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('members.password_reset', 'members', 'password_reset', 'Reset another member''s password directly')
ON CONFLICT (key) DO NOTHING;

-- Owner and Admin only — more sensitive than members.update.
UPDATE roles
SET permissions = array_cat(permissions, ARRAY['members.password_reset']),
    updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY['members.password_reset'])
)
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key = 'members.password_reset';

-- +goose StatementEnd