-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00124_seed_hrm_succession_permissions
--
-- Phase 10B permissions. Two resources, five keys:
--
--   hrm.succession.view              — critical positions (org design)
--   hrm.succession.manage            — designate, assess, nominate
--   hrm.succession.view_confidential — the 9-box, flight risk, nominations
--   hrm.development_plan.view        — an employee's own plan
--   hrm.development_plan.manage      — author and read anybody's plan
--
-- ⚠ NEITHER RESOURCE IS SCOPE-TIERED. Succession is org-wide talent
-- structure and both of its readers (the 9-box grid, the nomination bench)
-- need the whole population to mean anything — a nine-box drawn over your
-- own reports is a nine-dot scatter. Neither package calls ResolveScope, so
-- TestPermissions_ScopeTiersSeeded does not fire for either resource.
--
-- ⚠ view_confidential IS THE WHOLE CONFIDENTIALITY MODEL AT THE PERMISSION
-- LAYER, and it is deliberately NOT granted to manager.
--
-- The 9-box position, the flight-risk signals and the fact of a nomination
-- are judgements ABOUT a person that the person is not shown — and the
-- reader most likely to be an unwanted one is the subject's own manager,
-- who is usually the source of the assessment and the person the flight
-- risk is often about. This follows 9C's hrm.exits.interview_view
-- precedent exactly: administering something and reading what it contains
-- are different authorities.
--
-- The split is not only a permission. The confidential material lives in
-- different TABLES read by different REPOSITORY METHODS returning different
-- TYPES, so the subject's read path never selects a confidential column.
-- The permission is the outer gate; the query shape is what makes a leak
-- structurally impossible rather than a matter of remembering to filter.
--
-- Grant rationale:
--   • owner/admin: everything. Succession is an executive instrument.
--   • manager: succession.view (which roles are critical — org design they
--     need for planning) and development_plan.{view,manage} (coaching a
--     report is the manager's job). NOT view_confidential, NOT
--     succession.manage.
--   • member/viewer: development_plan.view only. The service restricts that
--     to the caller's OWN plan unless they also hold .manage, so this key
--     grants an employee sight of their own development and nothing else.
--
-- ⚠ development_plan.manage carries read-of-others as well as write. There
-- is no separate "read my team's plans" key, because coaching is an act
-- rather than a spectator sport: a manager who may read a report's
-- development plan may also be expected to edit it. Splitting them would
-- add a key with no distinct use.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.succession.view',
     'hrm.succession', 'view',
     'View critical position designations'),
    ('hrm.succession.manage',
     'hrm.succession', 'manage',
     'Designate critical positions, record talent assessments and nominate successors'),
    ('hrm.succession.view_confidential',
     'hrm.succession', 'view_confidential',
     'Read 9-box talent assessments, flight-risk signals and successor nominations'),
    ('hrm.development_plan.view',
     'hrm.development_plan', 'view',
     'View your own development plan'),
    ('hrm.development_plan.manage',
     'hrm.development_plan', 'manage',
     'Author development plans and read those of other employees')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.succession.view',
    'hrm.succession.manage',
    'hrm.succession.view_confidential',
    'hrm.development_plan.view',
    'hrm.development_plan.manage'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.succession.view',
    'hrm.development_plan.view',
    'hrm.development_plan.manage'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.development_plan.view'
]),
updated_at = NOW()
WHERE name IN ('member', 'viewer') AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY[
        'hrm.succession.view',
        'hrm.succession.manage',
        'hrm.succession.view_confidential',
        'hrm.development_plan.view',
        'hrm.development_plan.manage'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.succession.view',
    'hrm.succession.manage',
    'hrm.succession.view_confidential',
    'hrm.development_plan.view',
    'hrm.development_plan.manage'
);

-- +goose StatementEnd
