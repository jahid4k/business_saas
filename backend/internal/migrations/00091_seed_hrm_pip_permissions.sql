-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00091_seed_hrm_pip_permissions
--
-- Phase 5C part 2 permissions. One resource, hrm.pips, SCOPE-TIERED.
--
-- ⚠ The three scope tiers are MANDATORY. hrm_pips carries employee_id, so it
-- qualifies for the Phase 1 tiers exactly as hrm_goals (00083),
-- hrm_appraisals (00087) and hrm_feedback (00089) do — and
-- TestPermissions_ScopeTiersSeeded is all-or-nothing: seeding two of three
-- leaves holders of the missing tier resolving to ScopeNone, seeing nothing,
-- with no error raised.
--
-- A PIP is the most consequential record in this module: it is the document
-- that precedes a dismissal. The tiers are what stop a peer reading one.
--
-- Grant rationale:
--   • .view_own reaches 'member' because an employee must be able to read
--     their own plan. A PIP the subject cannot read is not a plan, it is a
--     paper trail, and every jurisdiction that recognises PIPs at all
--     requires the employee to have it.
--   • manager gets .view_team and .manage — a manager runs their report's
--     plan, writes check-ins, and records the outcome.
--   • .close is SEPARATE from .manage and does NOT reach 'manager'. Closing
--     a PIP as 'failed' is what triggers the draft-termination handoff, so
--     it is the moment the instrument stops being developmental. Owner/admin
--     only, deliberately: the same reasoning that keeps hrm.appraisals
--     .publish away from 'manager' in 00087.
--   • 'viewer' gets nothing, and org-created custom roles get nothing until
--     an admin grants explicitly — the 00077/00079/00081/00083/00087/00089
--     precedent.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.pips.view',      'hrm.pips', 'view',      'View performance improvement plans'),
    ('hrm.pips.manage',    'hrm.pips', 'manage',    'Create and administer performance improvement plans'),
    ('hrm.pips.close',     'hrm.pips', 'close',     'Record a PIP outcome, including the failed handoff to a draft termination'),
    ('hrm.pips.view_own',  'hrm.pips', 'view_own',  'View own performance improvement plan only'),
    ('hrm.pips.view_team', 'hrm.pips', 'view_team', 'View own and direct reports'' plans'),
    ('hrm.pips.view_all',  'hrm.pips', 'view_all',  'View all performance improvement plans in the organization')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.pips.view', 'hrm.pips.manage', 'hrm.pips.close', 'hrm.pips.view_all'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

-- manager runs the plan but does not close it: closing as 'failed' is what
-- creates the draft termination.
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.pips.view', 'hrm.pips.manage', 'hrm.pips.view_team'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

-- member reads their own plan and nothing else.
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.pips.view', 'hrm.pips.view_own'
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
        'hrm.pips.view', 'hrm.pips.manage', 'hrm.pips.close',
        'hrm.pips.view_own', 'hrm.pips.view_team', 'hrm.pips.view_all'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.pips.view', 'hrm.pips.manage', 'hrm.pips.close',
    'hrm.pips.view_own', 'hrm.pips.view_team', 'hrm.pips.view_all'
);

-- +goose StatementEnd
