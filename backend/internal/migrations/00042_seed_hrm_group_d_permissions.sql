-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00040_seed_hrm_group_d_permissions
--
-- Seeds HRM Group D permissions and assigns to system roles.
--
-- Permission matrix:
--   owner / admin  → all Group D permissions
--   manager        → view attendance + approve regularizations; view payroll
--   member         → view own attendance + payslips; submit regularizations
--   viewer         → view attendance + payroll (read-only)
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    -- D1: Attendance
    ('hrm.attendance.view',     'hrm.attendance', 'view',     'View attendance records and periods'),
    ('hrm.attendance.manage',   'hrm.attendance', 'manage',   'Create and update attendance records'),
    ('hrm.attendance.approve',  'hrm.attendance', 'approve',  'Approve regularization requests'),
    ('hrm.attendance.finalize', 'hrm.attendance', 'finalize', 'Finalize or lock an attendance period for payroll'),

    -- D2: Payslips / Payroll
    ('hrm.payroll.view',    'hrm.payroll', 'view',    'View payslip runs and individual payslips'),
    ('hrm.payroll.manage',  'hrm.payroll', 'manage',  'Create and update payslip runs'),
    ('hrm.payroll.compute', 'hrm.payroll', 'compute', 'Run the payroll computation for a period'),
    ('hrm.payroll.approve', 'hrm.payroll', 'approve', 'Approve a computed payroll run'),
    ('hrm.payroll.pay',     'hrm.payroll', 'pay',     'Mark a payroll run as paid')

ON CONFLICT (key) DO NOTHING;


-- Owner and Admin: full Group D access
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.attendance.view',   'hrm.attendance.manage', 'hrm.attendance.approve', 'hrm.attendance.finalize',
    'hrm.payroll.view',      'hrm.payroll.manage',    'hrm.payroll.compute',
    'hrm.payroll.approve',   'hrm.payroll.pay'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;


-- Manager: view + approve regularizations; view payroll (not approve/pay)
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.attendance.view', 'hrm.attendance.approve',
    'hrm.payroll.view'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;


-- Member: view own attendance; manage own regularizations; view own payslips
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.attendance.view',  'hrm.attendance.manage',
    'hrm.payroll.view'
]),
updated_at = NOW()
WHERE name = 'member' AND org_id IS NULL AND is_system = TRUE;


-- Viewer: read-only
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.attendance.view',
    'hrm.payroll.view'
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
        'hrm.attendance.view',   'hrm.attendance.manage', 'hrm.attendance.approve', 'hrm.attendance.finalize',
        'hrm.payroll.view',      'hrm.payroll.manage',    'hrm.payroll.compute',
        'hrm.payroll.approve',   'hrm.payroll.pay'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.attendance.view',   'hrm.attendance.manage', 'hrm.attendance.approve', 'hrm.attendance.finalize',
    'hrm.payroll.view',      'hrm.payroll.manage',    'hrm.payroll.compute',
    'hrm.payroll.approve',   'hrm.payroll.pay'
);

-- +goose StatementEnd
