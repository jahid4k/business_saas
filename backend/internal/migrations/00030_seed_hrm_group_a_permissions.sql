-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00028_seed_hrm_group_a_permissions
--
-- Seeds all HRM Extended Group A permissions into the
-- permissions table and assigns them to system roles.
--
-- Group A sub-modules covered:
--   A1. Salary components + structures + employee salary records
--   A2. Approval chain templates + instance actions
--   A3. Warning type configuration + escalation rules
--   A4. Document templates
--   A5. Shift configuration + assignments
--   A6. Holiday calendars + holidays + calendar assignments
--   A7. Employee contracts
--
-- Depends on: 00019_seed_hrm_permissions (resource pattern established)
-- Note: Group B migrations start at 00029 (this migration pushed them +1).
-- ============================================================

-- ──────────────────────────────────────────────────────────────
-- PART 1: Insert permissions
-- ──────────────────────────────────────────────────────────────

INSERT INTO permissions (key, resource, action, description) VALUES
    -- A1: Salary setup (components + structures — HR admin only)
    ('hrm.salary.view',             'hrm.salary',          'view',   'View salary components and structures'),
    ('hrm.salary.manage',           'hrm.salary',          'manage', 'Create and update salary components and structures'),
    -- A1: Employee salary records (sensitive — view own or manage team)
    ('hrm.salary.employee.view',    'hrm.salary.employee', 'view',   'View employee salary records'),
    ('hrm.salary.employee.manage',  'hrm.salary.employee', 'manage', 'Assign or update employee salary records'),

    -- A2: Approval chain templates (HR admin configures)
    ('hrm.approvals.view',          'hrm.approvals',       'view',   'View approval chain templates and instances'),
    ('hrm.approvals.manage',        'hrm.approvals',       'manage', 'Create and update approval chain templates'),
    -- A2: Approval action (anyone who is a designated approver)
    ('hrm.approvals.action',        'hrm.approvals',       'action', 'Approve or reject pending approval requests'),

    -- A3: Warning type configuration (HR admin only)
    ('hrm.warning_types.view',      'hrm.warning_types',   'view',   'View warning type and escalation rule configuration'),
    ('hrm.warning_types.manage',    'hrm.warning_types',   'manage', 'Create and update warning types and escalation rules'),

    -- A4: Document templates (HR admin configures)
    ('hrm.doc_templates.view',      'hrm.doc_templates',   'view',   'View document templates and preview filled content'),
    ('hrm.doc_templates.manage',    'hrm.doc_templates',   'manage', 'Create, update, and delete document templates'),

    -- A5: Work schedule and shift configuration
    ('hrm.shifts.view',             'hrm.shifts',          'view',   'View shift definitions and work schedule assignments'),
    ('hrm.shifts.manage',           'hrm.shifts',          'manage', 'Create and update shifts and work schedule assignments'),

    -- A6: Holiday calendars and assignments
    ('hrm.holidays.view',           'hrm.holidays',        'view',   'View holiday calendars and holiday entries'),
    ('hrm.holidays.manage',         'hrm.holidays',        'manage', 'Create and update holiday calendars, holidays, and assignments'),

    -- A7: Employee contracts
    ('hrm.contracts.view',          'hrm.contracts',       'view',   'View employee contracts'),
    ('hrm.contracts.manage',        'hrm.contracts',       'manage', 'Create, update, and deactivate employee contracts')

ON CONFLICT (key) DO NOTHING;


-- ──────────────────────────────────────────────────────────────
-- PART 2: Assign to system roles
--
-- Permission matrix:
--   owner  / admin   → ALL Group A permissions
--   manager          → view all config + approvals.action + salary employee view
--   member           → approvals.action + salary employee view + contracts view
--   viewer           → view config only (no employee data, no actions)
-- ──────────────────────────────────────────────────────────────

-- Owner and Admin: full Group A access
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.salary.view',            'hrm.salary.manage',
    'hrm.salary.employee.view',   'hrm.salary.employee.manage',
    'hrm.approvals.view',         'hrm.approvals.manage',       'hrm.approvals.action',
    'hrm.warning_types.view',     'hrm.warning_types.manage',
    'hrm.doc_templates.view',     'hrm.doc_templates.manage',
    'hrm.shifts.view',            'hrm.shifts.manage',
    'hrm.holidays.view',          'hrm.holidays.manage',
    'hrm.contracts.view',         'hrm.contracts.manage'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;


-- Manager: view all config, can act as approver, view team salary and contracts (read-only)
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.salary.view',
    'hrm.salary.employee.view',
    'hrm.approvals.view',         'hrm.approvals.action',
    'hrm.warning_types.view',
    'hrm.doc_templates.view',
    'hrm.shifts.view',
    'hrm.holidays.view',
    'hrm.contracts.view'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;


-- Member: self-service — participate in approvals, view own salary and contract
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.approvals.action',
    'hrm.salary.employee.view',
    'hrm.contracts.view',
    'hrm.shifts.view',
    'hrm.holidays.view'
]),
updated_at = NOW()
WHERE name = 'member' AND org_id IS NULL AND is_system = TRUE;


-- Viewer: read-only on config tables only — no employee salary data, no actions
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.salary.view',
    'hrm.approvals.view',
    'hrm.warning_types.view',
    'hrm.doc_templates.view',
    'hrm.shifts.view',
    'hrm.holidays.view',
    'hrm.contracts.view'
]),
updated_at = NOW()
WHERE name = 'viewer' AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Remove Group A permissions from all system roles
UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY[
        'hrm.salary.view',            'hrm.salary.manage',
        'hrm.salary.employee.view',   'hrm.salary.employee.manage',
        'hrm.approvals.view',         'hrm.approvals.manage',       'hrm.approvals.action',
        'hrm.warning_types.view',     'hrm.warning_types.manage',
        'hrm.doc_templates.view',     'hrm.doc_templates.manage',
        'hrm.shifts.view',            'hrm.shifts.manage',
        'hrm.holidays.view',          'hrm.holidays.manage',
        'hrm.contracts.view',         'hrm.contracts.manage'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

-- Remove Group A permissions from permissions table
DELETE FROM permissions WHERE key IN (
    'hrm.salary.view',            'hrm.salary.manage',
    'hrm.salary.employee.view',   'hrm.salary.employee.manage',
    'hrm.approvals.view',         'hrm.approvals.manage',       'hrm.approvals.action',
    'hrm.warning_types.view',     'hrm.warning_types.manage',
    'hrm.doc_templates.view',     'hrm.doc_templates.manage',
    'hrm.shifts.view',            'hrm.shifts.manage',
    'hrm.holidays.view',          'hrm.holidays.manage',
    'hrm.contracts.view',         'hrm.contracts.manage'
);

-- +goose StatementEnd
