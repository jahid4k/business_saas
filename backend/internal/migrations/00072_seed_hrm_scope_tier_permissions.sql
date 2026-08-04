-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00072_seed_hrm_scope_tier_permissions
--
-- Phase 1 (Resource-Level Permissions): seeds view_own/view_team/view_all
-- for the 12 HRM modules that return employee-level records. The existing
-- hrm.<resource>.view permission is untouched and stays the route-level gate
-- (RequirePermission) — these three new keys are checked by
-- authzSvc.ResolveScope inside the handler to decide query scope, not
-- whether the endpoint can be reached at all.
--
-- Grants below were checked against each role's ACTUAL current permission
-- set on this database before being written, not assumed from a template:
--
--   owner/admin → view_all on every resource (matches existing unscoped access).
--
--   manager     → view_team on every resource EXCEPT hrm.terminations, where
--                 manager already holds the base hrm.terminations.view
--                 permission today with no scoping. Granting view_all there
--                 (not view_team) preserves existing behavior rather than
--                 silently narrowing manager's termination visibility
--                 without explicit product sign-off.
--
--   member      → view_own, but ONLY where member already has some
--                 self-service involvement with the resource today:
--                 hrm.employees, hrm.leave, hrm.salary.employee,
--                 hrm.payroll, hrm.attendance, hrm.warnings, hrm.documents —
--                 member already holds base .view for all seven, so only
--                 view_own is added. hrm.resignations — member holds
--                 .manage (self-submit) but never held base .view; both are
--                 granted together here as a natural companion to the
--                 existing self-submit capability.
--                 Deliberately NOT granted to member: hrm.complaints
--                 (member can submit via .manage but the existing design
--                 never extended this to a "view my complaints" capability
--                 — member holds no hrm.complaints.view today),
--                 hrm.promotions / hrm.transfers / hrm.terminations (member
--                 holds zero permissions of any kind on these three today;
--                 granting view_own would be a new, undiscussed product
--                 capability, not a tightening of an existing one — and for
--                 terminations specifically there is an explicit existing
--                 code comment that employees cannot see their own
--                 termination records).
--
--   viewer      → view_all on every resource where viewer already holds
--                 base .view today (all except hrm.salary.employee and
--                 hrm.payroll, which viewer has never had access to — no
--                 change there). Viewer is a read-only oversight role with
--                 no hrm_employees record of its own; view_own/view_team
--                 would arbitrarily zero out a role that currently sees
--                 everything it's allowed to see, once list endpoints start
--                 enforcing scope. view_all preserves existing behavior.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.employees.view_own',        'hrm.employees',       'view_own',  'View own employee record'),
    ('hrm.employees.view_team',       'hrm.employees',       'view_team', 'View direct reports'' employee records'),
    ('hrm.employees.view_all',        'hrm.employees',       'view_all',  'View all employee records in the organization'),

    ('hrm.leave.view_own',            'hrm.leave',           'view_own',  'View own leave requests'),
    ('hrm.leave.view_team',           'hrm.leave',           'view_team', 'View direct reports'' leave requests'),
    ('hrm.leave.view_all',            'hrm.leave',           'view_all',  'View all leave requests in the organization'),

    ('hrm.salary.employee.view_own',  'hrm.salary.employee', 'view_own',  'View own salary records'),
    ('hrm.salary.employee.view_team', 'hrm.salary.employee', 'view_team', 'View direct reports'' salary records'),
    ('hrm.salary.employee.view_all',  'hrm.salary.employee', 'view_all',  'View all employee salary records in the organization'),

    ('hrm.payroll.view_own',          'hrm.payroll',         'view_own',  'View own payslips'),
    ('hrm.payroll.view_team',         'hrm.payroll',         'view_team', 'View direct reports'' payslips'),
    ('hrm.payroll.view_all',          'hrm.payroll',         'view_all',  'View all payslips in the organization'),

    ('hrm.attendance.view_own',       'hrm.attendance',      'view_own',  'View own attendance records'),
    ('hrm.attendance.view_team',      'hrm.attendance',      'view_team', 'View direct reports'' attendance records'),
    ('hrm.attendance.view_all',       'hrm.attendance',      'view_all',  'View all attendance records in the organization'),

    ('hrm.warnings.view_own',         'hrm.warnings',        'view_own',  'View own warning records'),
    ('hrm.warnings.view_team',        'hrm.warnings',        'view_team', 'View direct reports'' warning records'),
    ('hrm.warnings.view_all',         'hrm.warnings',        'view_all',  'View all warning records in the organization'),

    ('hrm.complaints.view_own',       'hrm.complaints',      'view_own',  'View complaints filed by the caller'),
    ('hrm.complaints.view_team',      'hrm.complaints',      'view_team', 'View complaints filed by direct reports'),
    ('hrm.complaints.view_all',       'hrm.complaints',      'view_all',  'View all complaint records in the organization'),

    ('hrm.documents.view_own',        'hrm.documents',       'view_own',  'View own employee documents'),
    ('hrm.documents.view_team',       'hrm.documents',       'view_team', 'View direct reports'' employee documents'),
    ('hrm.documents.view_all',        'hrm.documents',       'view_all',  'View all employee documents in the organization'),

    ('hrm.promotions.view_own',       'hrm.promotions',      'view_own',  'View own promotion records'),
    ('hrm.promotions.view_team',      'hrm.promotions',      'view_team', 'View direct reports'' promotion records'),
    ('hrm.promotions.view_all',       'hrm.promotions',      'view_all',  'View all promotion records in the organization'),

    ('hrm.transfers.view_own',        'hrm.transfers',       'view_own',  'View own transfer records'),
    ('hrm.transfers.view_team',       'hrm.transfers',       'view_team', 'View direct reports'' transfer records'),
    ('hrm.transfers.view_all',        'hrm.transfers',       'view_all',  'View all transfer records in the organization'),

    ('hrm.resignations.view_own',     'hrm.resignations',    'view_own',  'View own resignation record'),
    ('hrm.resignations.view_team',    'hrm.resignations',    'view_team', 'View direct reports'' resignation records'),
    ('hrm.resignations.view_all',     'hrm.resignations',    'view_all',  'View all resignation records in the organization'),

    ('hrm.terminations.view_own',     'hrm.terminations',    'view_own',  'View own termination record'),
    ('hrm.terminations.view_team',    'hrm.terminations',    'view_team', 'View direct reports'' termination records'),
    ('hrm.terminations.view_all',     'hrm.terminations',    'view_all',  'View all termination records in the organization')
ON CONFLICT (key) DO NOTHING;


-- Owner/admin: view_all on every resource
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.employees.view_all', 'hrm.leave.view_all', 'hrm.salary.employee.view_all',
    'hrm.payroll.view_all', 'hrm.attendance.view_all', 'hrm.warnings.view_all',
    'hrm.complaints.view_all', 'hrm.documents.view_all', 'hrm.promotions.view_all',
    'hrm.transfers.view_all', 'hrm.resignations.view_all', 'hrm.terminations.view_all'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;


-- Manager: view_team everywhere except terminations (view_all — preserves
-- manager's existing unscoped hrm.terminations.view today)
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.employees.view_team', 'hrm.leave.view_team', 'hrm.salary.employee.view_team',
    'hrm.payroll.view_team', 'hrm.attendance.view_team', 'hrm.warnings.view_team',
    'hrm.complaints.view_team', 'hrm.documents.view_team', 'hrm.promotions.view_team',
    'hrm.transfers.view_team', 'hrm.resignations.view_team', 'hrm.terminations.view_all'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;


-- Member: view_own where member already has self-service involvement today.
-- hrm.resignations.view is backfilled alongside view_own since member held
-- .manage (self-submit) but never held base .view for resignations.
-- Deliberately excluded: complaints, promotions, transfers, terminations
-- (see migration header).
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.employees.view_own', 'hrm.leave.view_own', 'hrm.salary.employee.view_own',
    'hrm.payroll.view_own', 'hrm.attendance.view_own', 'hrm.warnings.view_own',
    'hrm.documents.view_own', 'hrm.resignations.view', 'hrm.resignations.view_own'
]),
updated_at = NOW()
WHERE name = 'member' AND org_id IS NULL AND is_system = TRUE;


-- Viewer: view_all on every resource where viewer already holds base .view
-- today (all except hrm.salary.employee / hrm.payroll, which viewer has
-- never had access to — no change there).
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.employees.view_all', 'hrm.leave.view_all', 'hrm.attendance.view_all',
    'hrm.warnings.view_all', 'hrm.complaints.view_all', 'hrm.documents.view_all',
    'hrm.promotions.view_all', 'hrm.transfers.view_all', 'hrm.resignations.view_all',
    'hrm.terminations.view_all'
]),
updated_at = NOW()
WHERE name = 'viewer' AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Remove the resignations base-view backfill — the only base (non-tier)
-- permission this migration grants, and only ever to member.
UPDATE roles
SET permissions = array_remove(permissions, 'hrm.resignations.view'), updated_at = NOW()
WHERE name = 'member' AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY[
        'hrm.employees.view_own', 'hrm.employees.view_team', 'hrm.employees.view_all',
        'hrm.leave.view_own', 'hrm.leave.view_team', 'hrm.leave.view_all',
        'hrm.salary.employee.view_own', 'hrm.salary.employee.view_team', 'hrm.salary.employee.view_all',
        'hrm.payroll.view_own', 'hrm.payroll.view_team', 'hrm.payroll.view_all',
        'hrm.attendance.view_own', 'hrm.attendance.view_team', 'hrm.attendance.view_all',
        'hrm.warnings.view_own', 'hrm.warnings.view_team', 'hrm.warnings.view_all',
        'hrm.complaints.view_own', 'hrm.complaints.view_team', 'hrm.complaints.view_all',
        'hrm.documents.view_own', 'hrm.documents.view_team', 'hrm.documents.view_all',
        'hrm.promotions.view_own', 'hrm.promotions.view_team', 'hrm.promotions.view_all',
        'hrm.transfers.view_own', 'hrm.transfers.view_team', 'hrm.transfers.view_all',
        'hrm.resignations.view_own', 'hrm.resignations.view_team', 'hrm.resignations.view_all',
        'hrm.terminations.view_own', 'hrm.terminations.view_team', 'hrm.terminations.view_all'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.employees.view_own', 'hrm.employees.view_team', 'hrm.employees.view_all',
    'hrm.leave.view_own', 'hrm.leave.view_team', 'hrm.leave.view_all',
    'hrm.salary.employee.view_own', 'hrm.salary.employee.view_team', 'hrm.salary.employee.view_all',
    'hrm.payroll.view_own', 'hrm.payroll.view_team', 'hrm.payroll.view_all',
    'hrm.attendance.view_own', 'hrm.attendance.view_team', 'hrm.attendance.view_all',
    'hrm.warnings.view_own', 'hrm.warnings.view_team', 'hrm.warnings.view_all',
    'hrm.complaints.view_own', 'hrm.complaints.view_team', 'hrm.complaints.view_all',
    'hrm.documents.view_own', 'hrm.documents.view_team', 'hrm.documents.view_all',
    'hrm.promotions.view_own', 'hrm.promotions.view_team', 'hrm.promotions.view_all',
    'hrm.transfers.view_own', 'hrm.transfers.view_team', 'hrm.transfers.view_all',
    'hrm.resignations.view_own', 'hrm.resignations.view_team', 'hrm.resignations.view_all',
    'hrm.terminations.view_own', 'hrm.terminations.view_team', 'hrm.terminations.view_all'
);

-- +goose StatementEnd
