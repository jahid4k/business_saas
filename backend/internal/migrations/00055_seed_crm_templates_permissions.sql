-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00055_seed_crm_templates_permissions
--
-- Seed permissions for the CRM Templates module and add them
-- to the base roles.
-- ============================================================

INSERT INTO permissions (key, resource, action, description)
VALUES
    ('crm.templates.view',   'crm.templates', 'view',   'View CRM templates'),
    ('crm.templates.create', 'crm.templates', 'create', 'Create CRM templates'),
    ('crm.templates.update', 'crm.templates', 'update', 'Update CRM templates'),
    ('crm.templates.delete', 'crm.templates', 'delete', 'Delete CRM templates')
ON CONFLICT (key) DO NOTHING;

-- Owner and Admin: all CRM templates permissions
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'crm.templates.view', 'crm.templates.create', 'crm.templates.update', 'crm.templates.delete'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

-- Manager: view, create, update (no delete)
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'crm.templates.view', 'crm.templates.create', 'crm.templates.update'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

-- Member: view, create, update (no delete)
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'crm.templates.view', 'crm.templates.create', 'crm.templates.update'
]),
updated_at = NOW()
WHERE name = 'member' AND org_id IS NULL AND is_system = TRUE;

-- Viewer: view only
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'crm.templates.view'
]),
updated_at = NOW()
WHERE name = 'viewer' AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY[
        'crm.templates.view', 'crm.templates.create', 'crm.templates.update', 'crm.templates.delete'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd
