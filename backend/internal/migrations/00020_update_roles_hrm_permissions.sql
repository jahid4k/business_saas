-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00020_update_roles_hrm_permissions
-- Assigns HRM permissions to system roles following the same
-- principle as 00016_update_roles_crm_permissions.
--
-- Permission matrix:
--   owner  / admin   → all HRM permissions
--   manager          → view + manage leave approvals; no create/delete employees
--   member           → view employees/departments, request own leave
--   viewer           → read-only on employees, departments, positions, reports
-- ============================================================

-- Owner and Admin: full HRM access
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.employees.view',      'hrm.employees.create',    'hrm.employees.update',
    'hrm.employees.delete',    'hrm.employees.terminate',
    'hrm.departments.view',    'hrm.departments.create',  'hrm.departments.update',  'hrm.departments.delete',
    'hrm.positions.view',      'hrm.positions.create',    'hrm.positions.update',    'hrm.positions.delete',
    'hrm.leave.view',          'hrm.leave.create',        'hrm.leave.update',        'hrm.leave.delete',
    'hrm.leave.request',       'hrm.leave.approve',
    'hrm.reports.view'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

-- Manager: view + approve leave, view employees, no structural changes
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.employees.view',   'hrm.employees.update',
    'hrm.departments.view',
    'hrm.positions.view',
    'hrm.leave.view',       'hrm.leave.request',    'hrm.leave.approve',
    'hrm.reports.view'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

-- Member: view structure, request own leave only
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.employees.view',
    'hrm.departments.view',
    'hrm.positions.view',
    'hrm.leave.view',       'hrm.leave.request'
]),
updated_at = NOW()
WHERE name = 'member' AND org_id IS NULL AND is_system = TRUE;

-- Viewer: read-only
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.employees.view',
    'hrm.departments.view',
    'hrm.positions.view',
    'hrm.leave.view',
    'hrm.reports.view'
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
        'hrm.employees.view',      'hrm.employees.create',    'hrm.employees.update',
        'hrm.employees.delete',    'hrm.employees.terminate',
        'hrm.departments.view',    'hrm.departments.create',  'hrm.departments.update',  'hrm.departments.delete',
        'hrm.positions.view',      'hrm.positions.create',    'hrm.positions.update',    'hrm.positions.delete',
        'hrm.leave.view',          'hrm.leave.create',        'hrm.leave.update',        'hrm.leave.delete',
        'hrm.leave.request',       'hrm.leave.approve',
        'hrm.reports.view'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd
