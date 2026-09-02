-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00089_seed_hrm_feedback_permissions
--
-- Phase 5C part 1 permissions. One resource, hrm.feedback, SCOPE-TIERED.
--
-- ⚠ The three scope tiers are MANDATORY. hrm_feedback_requests carries
-- subject_employee_id, so it qualifies for the Phase 1 tiers exactly as
-- hrm_goals (00083) and hrm_appraisals (00087) do — and
-- TestPermissions_ScopeTiersSeeded is all-or-nothing: seeding two of three
-- leaves holders of the missing tier resolving to ScopeNone, seeing nothing,
-- with no error raised.
--
-- The action split is unusual here and is the point of the module, so it is
-- worth stating plainly. Two capabilities exist that look similar and are
-- not:
--
--   hrm.feedback.coordinate — see WHO WAS ASKED and who still owes a
--       response. Carries identity. Carries NO answer content.
--   hrm.feedback.view       — see WHAT WAS SAID about a subject, aggregated
--       and suppressed below the cycle's threshold. Carries content.
--       Carries NO identity for the anonymous relationship groups.
--
-- Nobody needs both, so nobody is granted a key that yields both. That is
-- what makes the anonymity structural rather than a filter somebody has to
-- remember: there is no permission in this system that returns a respondent's
-- name next to their answer.
--
-- Consequently 'owner' and 'admin' hold coordinate and manage but are NOT
-- exempt from suppression on the content path. An HR admin chasing responses
-- is doing coordination; an HR admin reading the feedback gets the same
-- anonymised aggregate the subject gets. An "admin can see everything"
-- exception would make the promise to respondents false, and a promise of
-- anonymity that is false for one role is false.
--
-- Grant rationale:
--   • .respond reaches 'member' — anyone can be asked for feedback. The
--     service narrows it to the request's own respondent.
--   • .view + .view_own reaches 'member' — a subject reads their own results.
--   • manager gets .view_team, seeing their reports' aggregates.
--   • .manage and .coordinate are owner/admin: running a campaign.
--   • 'viewer' gets nothing, and org-created custom roles get nothing until
--     an admin grants explicitly — the 00077/00079/00081/00083/00087
--     precedent.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.feedback.view',       'hrm.feedback', 'view',       'View anonymised 360 feedback aggregates'),
    ('hrm.feedback.manage',     'hrm.feedback', 'manage',     'Create and administer 360 feedback cycles'),
    ('hrm.feedback.coordinate', 'hrm.feedback', 'coordinate', 'See who was asked and who has responded, without answer content'),
    ('hrm.feedback.respond',    'hrm.feedback', 'respond',    'Answer a 360 feedback request addressed to you'),
    ('hrm.feedback.view_own',   'hrm.feedback', 'view_own',   'View own 360 feedback only'),
    ('hrm.feedback.view_team',  'hrm.feedback', 'view_team',  'View own and direct reports'' 360 feedback'),
    ('hrm.feedback.view_all',   'hrm.feedback', 'view_all',   'View all 360 feedback in the organization')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.feedback.view', 'hrm.feedback.manage', 'hrm.feedback.coordinate',
    'hrm.feedback.respond', 'hrm.feedback.view_all'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

-- manager reads their team's aggregates and answers their own requests.
-- Deliberately NOT .coordinate: chasing responses is running the campaign,
-- and a manager who could list who was asked about their own report would be
-- one join away from correlating a small group.
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.feedback.view', 'hrm.feedback.respond', 'hrm.feedback.view_team'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

-- member answers requests addressed to them and reads their own results.
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.feedback.view', 'hrm.feedback.respond', 'hrm.feedback.view_own'
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
        'hrm.feedback.view', 'hrm.feedback.manage', 'hrm.feedback.coordinate',
        'hrm.feedback.respond', 'hrm.feedback.view_own', 'hrm.feedback.view_team',
        'hrm.feedback.view_all'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.feedback.view', 'hrm.feedback.manage', 'hrm.feedback.coordinate',
    'hrm.feedback.respond', 'hrm.feedback.view_own', 'hrm.feedback.view_team',
    'hrm.feedback.view_all'
);

-- +goose StatementEnd
