-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00033_seed_hrm_group_b_permissions
--
-- Seeds HRM Extended Group B permissions and assigns them to
-- system roles.
--
-- Permission matrix:
--   owner / admin  → all Group B permissions
--   manager        → view + manage (create/edit) + apply (execute lifecycle changes)
--                    + process resignations (accept/reject)
--   member         → hrm.resignations.manage (submit own resignation only)
--   viewer         → view only (all 4 sub-modules)
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    -- B1: Promotions
    ('hrm.promotions.view',    'hrm.promotions',   'view',   'View employee promotion records'),
    ('hrm.promotions.manage',  'hrm.promotions',   'manage', 'Create and update employee promotions'),
    ('hrm.promotions.apply',   'hrm.promotions',   'apply',  'Apply an approved promotion to the employee record'),

    -- B2: Transfers
    ('hrm.transfers.view',     'hrm.transfers',    'view',   'View employee transfer records'),
    ('hrm.transfers.manage',   'hrm.transfers',    'manage', 'Create and update employee transfers'),
    ('hrm.transfers.apply',    'hrm.transfers',    'apply',  'Apply an approved transfer to the employee record'),

    -- B3: Resignations
    ('hrm.resignations.view',    'hrm.resignations', 'view',    'View all employee resignation records'),
    ('hrm.resignations.manage',  'hrm.resignations', 'manage',  'Submit and manage resignation records'),
    ('hrm.resignations.process', 'hrm.resignations', 'process', 'Accept, reject, or update resignation clearance (HR action)'),

    -- B4: Terminations
    ('hrm.terminations.view',    'hrm.terminations', 'view',   'View employee termination records'),
    ('hrm.terminations.manage',  'hrm.terminations', 'manage', 'Create and update employee terminations'),
    ('hrm.terminations.apply',   'hrm.terminations', 'apply',  'Apply an approved termination to the employee record')

ON CONFLICT (key) DO NOTHING;


-- Owner and Admin: full Group B access
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.promotions.view',    'hrm.promotions.manage',    'hrm.promotions.apply',
    'hrm.transfers.view',     'hrm.transfers.manage',     'hrm.transfers.apply',
    'hrm.resignations.view',  'hrm.resignations.manage',  'hrm.resignations.process',
    'hrm.terminations.view',  'hrm.terminations.manage',  'hrm.terminations.apply'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;


-- Manager: view + manage + process resignations of direct reports
-- Managers typically initiate promotions/transfers but an admin/owner applies them
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.promotions.view',   'hrm.promotions.manage',
    'hrm.transfers.view',    'hrm.transfers.manage',
    'hrm.resignations.view', 'hrm.resignations.process',
    'hrm.terminations.view'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;


-- Member: submit own resignation only
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.resignations.manage'
]),
updated_at = NOW()
WHERE name = 'member' AND org_id IS NULL AND is_system = TRUE;


-- Viewer: read-only across all Group B sub-modules
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.promotions.view',
    'hrm.transfers.view',
    'hrm.resignations.view',
    'hrm.terminations.view'
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
        'hrm.promotions.view',    'hrm.promotions.manage',    'hrm.promotions.apply',
        'hrm.transfers.view',     'hrm.transfers.manage',     'hrm.transfers.apply',
        'hrm.resignations.view',  'hrm.resignations.manage',  'hrm.resignations.process',
        'hrm.terminations.view',  'hrm.terminations.manage',  'hrm.terminations.apply'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.promotions.view',    'hrm.promotions.manage',    'hrm.promotions.apply',
    'hrm.transfers.view',     'hrm.transfers.manage',     'hrm.transfers.apply',
    'hrm.resignations.view',  'hrm.resignations.manage',  'hrm.resignations.process',
    'hrm.terminations.view',  'hrm.terminations.manage',  'hrm.terminations.apply'
);

-- +goose StatementEnd
