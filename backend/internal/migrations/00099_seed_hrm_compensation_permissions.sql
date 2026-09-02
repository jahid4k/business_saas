-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00099_seed_hrm_compensation_permissions
--
-- Phase 7B permissions. Three resources:
--
--   hrm.compensation_config  — bands + merit matrix cells, NOT scope-tiered
--   hrm.salary_revisions     — cycles + per-employee revisions, SCOPE-TIERED
--   hrm.bonuses               — per-employee bonuses, SCOPE-TIERED
--
-- hrm.compensation_config covers BOTH hrm_compensation_bands and
-- hrm_merit_matrix_cells under one resource — the hrm.rating_scales
-- precedent (00087), where scale + level catalog tables share one resource
-- because they are always administered together. Neither table carries
-- employee_id, so — as with hrm.rating_scales and hrm.goal_cycles before it —
-- do NOT call ResolveScope for this resource; a tier would imply a
-- per-employee filter that cannot exist on catalog data.
--
-- hrm.salary_revisions and hrm.bonuses both carry employee_id on their row
-- tables (hrm_salary_revisions, hrm_bonuses) and their three scope tiers are
-- MANDATORY — TestPermissions_ScopeTiersSeeded is all-or-nothing, and this is
-- compensation data: the single most sensitive field a scope hole could leak.
--
-- No separate .approve permission on either resource. Deciding a cycle or a
-- bonus's approval instance goes through hrm.approvals.action
-- (POST .../approval-instances/:id/approve|reject) exactly as promotions,
-- transfers, terminations, warnings, awards and offers already do — the
-- decision is authorized once, generically, by the approvals module itself.
--
-- Grant rationale:
--   • .manage (create/compute/submit cycles and bonuses) is owner/admin only.
--     Unlike hrm.goals.set_own, there is no self-service path here — an
--     employee proposes nothing about their own pay.
--   • manager gets view_team on both tiered resources — visibility into a
--     report's compensation without the ability to originate a change,
--     mirroring hrm.appraisals where manager reviews but never manages.
--   • member gets view_own on both — seeing your own proposed revision or
--     bonus before it is applied/paid.
--   • 'viewer' gets nothing, the 00087/00089/00091/00093/00095 precedent.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.compensation_config.view',    'hrm.compensation_config', 'view',      'View compensation bands and merit matrix'),
    ('hrm.compensation_config.manage',  'hrm.compensation_config', 'manage',    'Administer compensation bands and merit matrix'),

    ('hrm.salary_revisions.view',       'hrm.salary_revisions',    'view',      'View salary revision cycles'),
    ('hrm.salary_revisions.manage',     'hrm.salary_revisions',    'manage',    'Create, compute and submit salary revision cycles'),
    ('hrm.salary_revisions.view_own',   'hrm.salary_revisions',    'view_own',  'View own salary revisions only'),
    ('hrm.salary_revisions.view_team',  'hrm.salary_revisions',    'view_team', 'View own and direct reports'' salary revisions'),
    ('hrm.salary_revisions.view_all',   'hrm.salary_revisions',    'view_all',  'View all salary revisions in the organization'),

    ('hrm.bonuses.view',                'hrm.bonuses',              'view',      'View bonuses'),
    ('hrm.bonuses.manage',              'hrm.bonuses',              'manage',    'Create and submit bonuses'),
    ('hrm.bonuses.view_own',            'hrm.bonuses',              'view_own',  'View own bonuses only'),
    ('hrm.bonuses.view_team',           'hrm.bonuses',              'view_team', 'View own and direct reports'' bonuses'),
    ('hrm.bonuses.view_all',            'hrm.bonuses',              'view_all',  'View all bonuses in the organization')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.compensation_config.view', 'hrm.compensation_config.manage',
    'hrm.salary_revisions.view', 'hrm.salary_revisions.manage', 'hrm.salary_revisions.view_all',
    'hrm.bonuses.view', 'hrm.bonuses.manage', 'hrm.bonuses.view_all'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.compensation_config.view',
    'hrm.salary_revisions.view', 'hrm.salary_revisions.view_team',
    'hrm.bonuses.view', 'hrm.bonuses.view_team'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.salary_revisions.view', 'hrm.salary_revisions.view_own',
    'hrm.bonuses.view', 'hrm.bonuses.view_own'
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
        'hrm.compensation_config.view', 'hrm.compensation_config.manage',
        'hrm.salary_revisions.view', 'hrm.salary_revisions.manage',
        'hrm.salary_revisions.view_own', 'hrm.salary_revisions.view_team', 'hrm.salary_revisions.view_all',
        'hrm.bonuses.view', 'hrm.bonuses.manage',
        'hrm.bonuses.view_own', 'hrm.bonuses.view_team', 'hrm.bonuses.view_all'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.compensation_config.view', 'hrm.compensation_config.manage',
    'hrm.salary_revisions.view', 'hrm.salary_revisions.manage',
    'hrm.salary_revisions.view_own', 'hrm.salary_revisions.view_team', 'hrm.salary_revisions.view_all',
    'hrm.bonuses.view', 'hrm.bonuses.manage',
    'hrm.bonuses.view_own', 'hrm.bonuses.view_team', 'hrm.bonuses.view_all'
);

-- +goose StatementEnd
