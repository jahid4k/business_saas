-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00087_seed_hrm_appraisal_permissions
--
-- Phase 5B part 2 permissions. Two resources:
--
--   hrm.appraisals     — the appraisals themselves, SCOPE-TIERED
--   hrm.rating_scales  — org-level rating configuration, NOT scope-tiered
--
-- ⚠ The three scope tiers on hrm.appraisals are MANDATORY. Appraisal rows
-- carry an employee_id, so they qualify for the Phase 1 tiers exactly as
-- hrm_goals does (migration 00083) — and TestPermissions_ScopeTiersSeeded is
-- all-or-nothing: seeding two of three leaves holders of the missing tier
-- silently resolving to ScopeNone, seeing nothing, with no error.
--
-- This module is the one the Section 9 primitive note singles out: "appraisal
-- draft leakage ... [is a] trust and legal issue, not UX polish". An
-- unpublished appraisal is the single most sensitive employee record this
-- system holds, and the tiers are what stop a peer reading one.
--
-- As with hrm.goal_cycles in 00083, do NOT call ResolveScope for
-- hrm.rating_scales — a scale is org-level configuration with no employee_id
-- to filter on, so tiers there would imply a filter that cannot exist.
--
-- Grant rationale:
--   • .respond reaches 'member' because the appraisee writes their own
--     self-review. The service narrows it to the appraisal's own employee.
--   • .review reaches 'manager' — writing the manager-review half.
--   • .calibrate and .publish are owner/admin only: calibration overrides a
--     manager's rating and publication is irreversible.
--   • .manage (create cycles, instantiate) is owner/admin.
--   • 'viewer' gets nothing, and org-created custom roles get nothing until
--     an admin grants explicitly — the 00077/00079/00081/00083 precedent.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.appraisals.view',       'hrm.appraisals',    'view',      'View appraisals'),
    ('hrm.appraisals.manage',     'hrm.appraisals',    'manage',    'Create appraisal cycles and instantiate appraisals'),
    ('hrm.appraisals.respond',    'hrm.appraisals',    'respond',   'Complete your own self-review'),
    ('hrm.appraisals.review',     'hrm.appraisals',    'review',    'Complete the manager-review half of an appraisal'),
    ('hrm.appraisals.calibrate',  'hrm.appraisals',    'calibrate', 'Adjust ratings during calibration'),
    ('hrm.appraisals.publish',    'hrm.appraisals',    'publish',   'Publish an appraisal to its employee'),
    ('hrm.appraisals.view_own',   'hrm.appraisals',    'view_own',  'View own appraisals only'),
    ('hrm.appraisals.view_team',  'hrm.appraisals',    'view_team', 'View own and direct reports'' appraisals'),
    ('hrm.appraisals.view_all',   'hrm.appraisals',    'view_all',  'View all appraisals in the organization'),
    ('hrm.rating_scales.view',    'hrm.rating_scales', 'view',      'View rating scales'),
    ('hrm.rating_scales.manage',  'hrm.rating_scales', 'manage',    'Create and administer rating scales')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.appraisals.view', 'hrm.appraisals.manage', 'hrm.appraisals.respond',
    'hrm.appraisals.review', 'hrm.appraisals.calibrate', 'hrm.appraisals.publish',
    'hrm.appraisals.view_all',
    'hrm.rating_scales.view', 'hrm.rating_scales.manage'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

-- manager reviews their reports and sees their team, but never calibrates or
-- publishes — calibration exists precisely to adjust manager ratings.
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.appraisals.view', 'hrm.appraisals.respond', 'hrm.appraisals.review',
    'hrm.appraisals.view_team',
    'hrm.rating_scales.view'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

-- member writes their own self-review and sees only their own appraisals.
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.appraisals.view', 'hrm.appraisals.respond', 'hrm.appraisals.view_own',
    'hrm.rating_scales.view'
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
        'hrm.appraisals.view', 'hrm.appraisals.manage', 'hrm.appraisals.respond',
        'hrm.appraisals.review', 'hrm.appraisals.calibrate', 'hrm.appraisals.publish',
        'hrm.appraisals.view_own', 'hrm.appraisals.view_team', 'hrm.appraisals.view_all',
        'hrm.rating_scales.view', 'hrm.rating_scales.manage'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.appraisals.view', 'hrm.appraisals.manage', 'hrm.appraisals.respond',
    'hrm.appraisals.review', 'hrm.appraisals.calibrate', 'hrm.appraisals.publish',
    'hrm.appraisals.view_own', 'hrm.appraisals.view_team', 'hrm.appraisals.view_all',
    'hrm.rating_scales.view', 'hrm.rating_scales.manage'
);

-- +goose StatementEnd
