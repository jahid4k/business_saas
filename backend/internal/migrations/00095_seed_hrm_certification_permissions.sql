-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00095_seed_hrm_certification_permissions
--
-- Phase 6B permissions. Two resources:
--
--   hrm.certifications — a person's credentials. SCOPE-TIERED.
--   hrm.skills         — the org taxonomy AND employee skills. SCOPE-TIERED.
--
-- ⚠ Both sets of three scope tiers are MANDATORY. hrm_employee_certifications
-- and hrm_employee_skills both carry an employee_id, so they qualify for the
-- Phase 1 tiers exactly as hrm_goals (00083), hrm_appraisals (00087),
-- hrm_feedback (00089), hrm_pips (00091) and hrm_enrollments (00093) do — and
-- TestPermissions_ScopeTiersSeeded is all-or-nothing: seeding two of three
-- leaves holders of the missing tier resolving to ScopeNone, seeing nothing,
-- with no error raised.
--
-- The CATALOGUE halves (hrm_certifications, hrm_skills) have no employee_id
-- and are never passed to ResolveScope. They ride on the same resource key as
-- their per-employee halves rather than getting resources of their own,
-- because a catalogue nobody can read is useless and splitting them would
-- force three more tier keys for a filter that cannot exist. The .manage
-- action covers catalogue administration in both.
--
-- Auto-assignment rules are administered under hrm.courses.manage — they
-- assign COURSES, they hold no employee data, and giving them their own
-- resource would be a key nobody ever grants separately.
--
-- Grant rationale:
--   • .view + .view_own reaches 'member': an employee must be able to see
--     their own credentials and skills, not least because they are the person
--     who has to renew them.
--   • manager gets .view_team — a manager needs to know which of their
--     reports' certifications are lapsing.
--   • .manage (issue, revoke, administer the catalogue) is owner/admin. Issuing
--     a credential is an assertion the organization stands behind, which is
--     why 'manager' does not get it.
--   • 'viewer' gets nothing, and org-created custom roles get nothing until
--     an admin grants explicitly — the 00077 through 00093 precedent.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.certifications.view',      'hrm.certifications', 'view',      'View certifications and the certification catalogue'),
    ('hrm.certifications.manage',    'hrm.certifications', 'manage',    'Administer the catalogue, issue and revoke credentials'),
    ('hrm.certifications.view_own',  'hrm.certifications', 'view_own',  'View own certifications only'),
    ('hrm.certifications.view_team', 'hrm.certifications', 'view_team', 'View own and direct reports'' certifications'),
    ('hrm.certifications.view_all',  'hrm.certifications', 'view_all',  'View all certifications in the organization'),
    ('hrm.skills.view',              'hrm.skills',         'view',      'View the skills taxonomy and employee skills'),
    ('hrm.skills.manage',            'hrm.skills',         'manage',    'Administer the skills taxonomy and record employee skills'),
    ('hrm.skills.view_own',          'hrm.skills',         'view_own',  'View own skills only'),
    ('hrm.skills.view_team',         'hrm.skills',         'view_team', 'View own and direct reports'' skills'),
    ('hrm.skills.view_all',          'hrm.skills',         'view_all',  'View all employee skills in the organization')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.certifications.view', 'hrm.certifications.manage', 'hrm.certifications.view_all',
    'hrm.skills.view', 'hrm.skills.manage', 'hrm.skills.view_all'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

-- manager sees their team's credentials and skills — knowing which reports'
-- certifications lapse is the point — but does not issue them.
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.certifications.view', 'hrm.certifications.view_team',
    'hrm.skills.view', 'hrm.skills.view_team'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

-- member sees their own, since they are the one who has to renew them.
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.certifications.view', 'hrm.certifications.view_own',
    'hrm.skills.view', 'hrm.skills.view_own'
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
        'hrm.certifications.view', 'hrm.certifications.manage',
        'hrm.certifications.view_own', 'hrm.certifications.view_team', 'hrm.certifications.view_all',
        'hrm.skills.view', 'hrm.skills.manage',
        'hrm.skills.view_own', 'hrm.skills.view_team', 'hrm.skills.view_all'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.certifications.view', 'hrm.certifications.manage',
    'hrm.certifications.view_own', 'hrm.certifications.view_team', 'hrm.certifications.view_all',
    'hrm.skills.view', 'hrm.skills.manage',
    'hrm.skills.view_own', 'hrm.skills.view_team', 'hrm.skills.view_all'
);

-- +goose StatementEnd
