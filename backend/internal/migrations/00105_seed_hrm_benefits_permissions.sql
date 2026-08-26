-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00105_seed_hrm_benefits_permissions
--
-- Phase 7D (benefits half) permissions. Two resources:
--
--   hrm.benefit_plans        — plans + tiers catalog, NOT scope-tiered
--   hrm.benefit_enrollments  — enrollments + dependents, SCOPE-TIERED
--
-- hrm.benefit_plans covers BOTH hrm_benefit_plans and hrm_benefit_tiers
-- under one resource, the hrm.rating_scales / hrm.compensation_config
-- precedent (00087/00099) — neither carries employee_id, so no tiers.
--
-- hrm.benefit_enrollments carries the three mandatory scope tiers —
-- TestPermissions_ScopeTiersSeeded is all-or-nothing — because
-- hrm_benefit_enrollments and hrm_dependents both carry employee_id and
-- this is health/benefits data, arguably as sensitive as compensation.
--
-- No .approve permission — unlike loans/bonuses/revisions, enrollment is
-- employee self-service (the whole point of an "enrollment window"), not an
-- approval-gated workflow. Verifying a dependent is its own action
-- (.verify_dependent) because it is the one enrollment-adjacent action that
-- is NOT self-service — an employee cannot verify their own dependent.
--
-- Grant rationale:
--   • .manage (plans/tiers) and .verify_dependent are owner/admin only.
--   • member gets .enroll_self + view_own — the whole point of self-service
--     enrollment. This mirrors hrm.goals.set_own (00082): the route cannot
--     express "is this your own enrollment", so the SERVICE narrows it —
--     .enroll_self does not, by itself, let a member enroll someone else.
--   • manager gets view + view_team, no manage/enroll_self/verify —
--     visibility into a report's coverage without originating anything.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.benefit_plans.view',   'hrm.benefit_plans', 'view',   'View benefit plans and tiers'),
    ('hrm.benefit_plans.manage', 'hrm.benefit_plans', 'manage', 'Administer benefit plans and tiers'),

    ('hrm.benefit_enrollments.view',             'hrm.benefit_enrollments', 'view',             'View benefit enrollments'),
    ('hrm.benefit_enrollments.enroll_self',      'hrm.benefit_enrollments', 'enroll_self',      'Enroll yourself in a benefit plan'),
    ('hrm.benefit_enrollments.verify_dependent', 'hrm.benefit_enrollments', 'verify_dependent', 'Verify a dependent record'),
    ('hrm.benefit_enrollments.view_own',         'hrm.benefit_enrollments', 'view_own',         'View own enrollments only'),
    ('hrm.benefit_enrollments.view_team',        'hrm.benefit_enrollments', 'view_team',        'View own and direct reports'' enrollments'),
    ('hrm.benefit_enrollments.view_all',         'hrm.benefit_enrollments', 'view_all',         'View all enrollments in the organization')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.benefit_plans.view', 'hrm.benefit_plans.manage',
    'hrm.benefit_enrollments.view', 'hrm.benefit_enrollments.enroll_self',
    'hrm.benefit_enrollments.verify_dependent', 'hrm.benefit_enrollments.view_all'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.benefit_plans.view',
    'hrm.benefit_enrollments.view', 'hrm.benefit_enrollments.view_team'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.benefit_plans.view',
    'hrm.benefit_enrollments.view', 'hrm.benefit_enrollments.enroll_self', 'hrm.benefit_enrollments.view_own'
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
        'hrm.benefit_plans.view', 'hrm.benefit_plans.manage',
        'hrm.benefit_enrollments.view', 'hrm.benefit_enrollments.enroll_self',
        'hrm.benefit_enrollments.verify_dependent',
        'hrm.benefit_enrollments.view_own', 'hrm.benefit_enrollments.view_team', 'hrm.benefit_enrollments.view_all'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.benefit_plans.view', 'hrm.benefit_plans.manage',
    'hrm.benefit_enrollments.view', 'hrm.benefit_enrollments.enroll_self',
    'hrm.benefit_enrollments.verify_dependent',
    'hrm.benefit_enrollments.view_own', 'hrm.benefit_enrollments.view_team', 'hrm.benefit_enrollments.view_all'
);

-- +goose StatementEnd
