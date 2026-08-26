-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00101_seed_hrm_loans_reimbursements_permissions
--
-- Phase 7C permissions. Two resources, both scope-tiered — every row in
-- hrm_loans and hrm_reimbursements carries an employee_id:
--
--   hrm.loans           — view/manage/disburse/foreclose + view_own/team/all
--   hrm.reimbursements  — view/manage + view_own/team/all
--
-- No separate .approve permission on either — deciding a submitted loan or
-- reimbursement's approval instance goes through hrm.approvals.action, the
-- 00099/promotions/transfers precedent.
--
-- .disburse and .foreclose are their OWN permissions on hrm.loans, distinct
-- from .manage, because both are real money-movement events after a decision
-- has already been made — the same reasoning that kept ApplyCycle (00099)
-- a separate call from the approval decision. An org that lets a manager
-- create/submit loan requests need not also trust them to release funds or
-- write off a balance early.
--
-- Grant rationale, mirroring 00099:
--   • .manage/.disburse/.foreclose are owner/admin only — there is no
--     self-service path (an employee proposes nothing about their own loan
--     terms), and disbursing/foreclosing are the sharpest money-movement
--     actions in this slice.
--   • manager gets view + view_team on both resources — visibility into a
--     report's loan/reimbursement status without originating anything.
--   • member gets view + view_own on both.
--   • 'viewer' gets nothing, the 00087/.../00099 precedent.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.loans.view',            'hrm.loans', 'view',       'View employee loans'),
    ('hrm.loans.manage',          'hrm.loans', 'manage',     'Create and submit loan requests'),
    ('hrm.loans.disburse',        'hrm.loans', 'disburse',   'Disburse an approved loan and generate its amortization schedule'),
    ('hrm.loans.foreclose',       'hrm.loans', 'foreclose',  'Foreclose an active loan'),
    ('hrm.loans.view_own',        'hrm.loans', 'view_own',   'View own loans only'),
    ('hrm.loans.view_team',       'hrm.loans', 'view_team',  'View own and direct reports'' loans'),
    ('hrm.loans.view_all',        'hrm.loans', 'view_all',   'View all loans in the organization'),

    ('hrm.reimbursements.view',      'hrm.reimbursements', 'view',      'View reimbursements'),
    ('hrm.reimbursements.manage',    'hrm.reimbursements', 'manage',    'Create and submit reimbursements'),
    ('hrm.reimbursements.view_own',  'hrm.reimbursements', 'view_own',  'View own reimbursements only'),
    ('hrm.reimbursements.view_team', 'hrm.reimbursements', 'view_team', 'View own and direct reports'' reimbursements'),
    ('hrm.reimbursements.view_all',  'hrm.reimbursements', 'view_all',  'View all reimbursements in the organization')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.loans.view', 'hrm.loans.manage', 'hrm.loans.disburse', 'hrm.loans.foreclose', 'hrm.loans.view_all',
    'hrm.reimbursements.view', 'hrm.reimbursements.manage', 'hrm.reimbursements.view_all'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.loans.view', 'hrm.loans.view_team',
    'hrm.reimbursements.view', 'hrm.reimbursements.view_team'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.loans.view', 'hrm.loans.view_own',
    'hrm.reimbursements.view', 'hrm.reimbursements.view_own'
]),
updated_at = NOW()
WHERE name = 'member' AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY[
        'hrm.loans.view', 'hrm.loans.manage', 'hrm.loans.disburse', 'hrm.loans.foreclose',
        'hrm.loans.view_own', 'hrm.loans.view_team', 'hrm.loans.view_all',
        'hrm.reimbursements.view', 'hrm.reimbursements.manage',
        'hrm.reimbursements.view_own', 'hrm.reimbursements.view_team', 'hrm.reimbursements.view_all'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.loans.view', 'hrm.loans.manage', 'hrm.loans.disburse', 'hrm.loans.foreclose',
    'hrm.loans.view_own', 'hrm.loans.view_team', 'hrm.loans.view_all',
    'hrm.reimbursements.view', 'hrm.reimbursements.manage',
    'hrm.reimbursements.view_own', 'hrm.reimbursements.view_team', 'hrm.reimbursements.view_all'
);

-- +goose StatementEnd
