-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00093_seed_hrm_learning_permissions
--
-- Phase 6A permissions. Two resources:
--
--   hrm.courses      — the catalogue and its content. NOT scope-tiered.
--   hrm.enrollments  — a learner's record. SCOPE-TIERED.
--
-- ⚠ The three scope tiers on hrm.enrollments are MANDATORY. Enrollment rows
-- carry an employee_id, so they qualify for the Phase 1 tiers exactly as
-- hrm_goals (00083), hrm_appraisals (00087), hrm_feedback (00089) and
-- hrm_pips (00091) do — and TestPermissions_ScopeTiersSeeded is
-- all-or-nothing: seeding two of three leaves holders of the missing tier
-- resolving to ScopeNone, seeing nothing, with no error raised.
--
-- Do NOT call ResolveScope for hrm.courses. A course is org-level content
-- with no employee_id to filter on — scope.Predicate hard-codes
-- FROM hrm_employees — so tiers there would force three more seeded keys
-- while implying a filter that cannot exist. Same reasoning that kept
-- hrm.goal_cycles (00083) and hrm.rating_scales (00087) untiered.
--
-- Grant rationale:
--   • .view on courses reaches every role including 'viewer': a course
--     catalogue is not sensitive, and a learner must be able to see what
--     exists before being assigned it.
--   • .manage on courses (authoring, versioning, publishing) is owner/admin.
--     Note that publishing is NOT split into its own key — unlike
--     hrm.appraisals.publish, publishing a course version is reversible by
--     publishing another, and it discloses nothing about a person.
--   • .enroll_self reaches 'member', which is what makes an optional
--     catalogue self-service. The service narrows it to the caller's own
--     employee record.
--   • .attempt reaches 'member' — sitting a quiz. Narrowed by the service to
--     the enrollment's own learner, since the route gate cannot express
--     "is this YOUR enrollment".
--   • .manage on enrollments (assigning others, cancelling) reaches
--     'manager', who assigns training to their reports; the scope tier is
--     what stops them assigning outside their reporting line.
--   • .grade is owner/admin only. It exists for the manual-override path and
--     is the only key that can read hrm_quiz_answer_keys through an
--     endpoint. Deliberately away from 'manager': a manager who could read
--     the answer key for their report's quiz has defeated the assessment.
--   • 'viewer' gets courses.view only, and org-created custom roles get
--     nothing until an admin grants explicitly — the
--     00077/00079/00081/00083/00087/00089/00091 precedent.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.courses.view',          'hrm.courses',     'view',        'View the course catalogue and content'),
    ('hrm.courses.manage',        'hrm.courses',     'manage',      'Author, version and publish courses'),
    ('hrm.enrollments.view',      'hrm.enrollments', 'view',        'View course enrollments'),
    ('hrm.enrollments.manage',    'hrm.enrollments', 'manage',      'Assign and cancel enrollments for others'),
    ('hrm.enrollments.enroll_self','hrm.enrollments','enroll_self', 'Enrol yourself on an open course'),
    ('hrm.enrollments.attempt',   'hrm.enrollments', 'attempt',     'Progress lessons and sit quizzes on your own enrollment'),
    ('hrm.enrollments.grade',     'hrm.enrollments', 'grade',       'Read answer keys and override a quiz grade'),
    ('hrm.enrollments.view_own',  'hrm.enrollments', 'view_own',    'View own enrollments only'),
    ('hrm.enrollments.view_team', 'hrm.enrollments', 'view_team',   'View own and direct reports'' enrollments'),
    ('hrm.enrollments.view_all',  'hrm.enrollments', 'view_all',    'View all enrollments in the organization')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.courses.view', 'hrm.courses.manage',
    'hrm.enrollments.view', 'hrm.enrollments.manage', 'hrm.enrollments.enroll_self',
    'hrm.enrollments.attempt', 'hrm.enrollments.grade', 'hrm.enrollments.view_all'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

-- manager assigns training to their reports and sees their team, but never
-- reads an answer key — that would defeat the assessment they are assigning.
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.courses.view',
    'hrm.enrollments.view', 'hrm.enrollments.manage', 'hrm.enrollments.enroll_self',
    'hrm.enrollments.attempt', 'hrm.enrollments.view_team'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

-- member takes courses and sees only their own record.
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.courses.view',
    'hrm.enrollments.view', 'hrm.enrollments.enroll_self', 'hrm.enrollments.attempt',
    'hrm.enrollments.view_own'
]),
updated_at = NOW()
WHERE name = 'member' AND org_id IS NULL AND is_system = TRUE;

-- viewer sees the catalogue and nothing about anybody's progress.
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.courses.view'
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
        'hrm.courses.view', 'hrm.courses.manage',
        'hrm.enrollments.view', 'hrm.enrollments.manage', 'hrm.enrollments.enroll_self',
        'hrm.enrollments.attempt', 'hrm.enrollments.grade',
        'hrm.enrollments.view_own', 'hrm.enrollments.view_team', 'hrm.enrollments.view_all'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.courses.view', 'hrm.courses.manage',
    'hrm.enrollments.view', 'hrm.enrollments.manage', 'hrm.enrollments.enroll_self',
    'hrm.enrollments.attempt', 'hrm.enrollments.grade',
    'hrm.enrollments.view_own', 'hrm.enrollments.view_team', 'hrm.enrollments.view_all'
);

-- +goose StatementEnd
