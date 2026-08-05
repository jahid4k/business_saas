-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00081_seed_hrm_recruitment_selection_permissions
--
-- Phase 4B permissions. Three resources:
--
--   hrm.interviews.*  — scheduling, panelists
--   hrm.offers.*       — compensation offers (financial, approval-gated)
--   hrm.referrals.*    — referral bonus-program lifecycle (financial)
--
-- hrm.interviews.scorecard is granted broadly (through member) then
-- NARROWED by the service to actual panelists — the platform.checklists
-- .complete precedent from Phase 3: the route gate cannot express "is this
-- actually your panel assignment", so it does not try. This is deliberately
-- separate from hrm.interviews.manage (owner/admin only, scheduling/
-- panelist assignment) so an individual-contributor panelist can submit
-- their own scorecard without holding interview-management rights.
--
-- hire conversion reuses the existing hrm.candidates.manage permission
-- (an application-lifecycle action, consistent with move/reject/withdraw
-- already being gated by it) — no new key, to avoid permission sprawl.
--
-- As with 00079 and 00081's predecessors: member/viewer get nothing beyond
-- what's listed, and org-created custom roles get nothing until an admin
-- grants it explicitly — these are new capabilities, not a restoration of
-- prior behaviour.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.interviews.view',       'hrm.interviews', 'view',       'View scheduled interviews and panelists'),
    ('hrm.interviews.manage',     'hrm.interviews', 'manage',     'Schedule interviews and assign panelists'),
    ('hrm.interviews.scorecard',  'hrm.interviews', 'scorecard',  'Submit an interview scorecard for a panel the caller is assigned to'),
    ('hrm.offers.view',           'hrm.offers',     'view',       'View compensation offers'),
    ('hrm.offers.manage',         'hrm.offers',     'manage',     'Create, submit, and administer compensation offers'),
    ('hrm.referrals.view',        'hrm.referrals',  'view',       'View referral records'),
    ('hrm.referrals.manage',      'hrm.referrals',  'manage',     'Create and administer referral bonus records')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.interviews.view', 'hrm.interviews.manage', 'hrm.interviews.scorecard',
    'hrm.offers.view', 'hrm.offers.manage',
    'hrm.referrals.view', 'hrm.referrals.manage'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.interviews.view', 'hrm.interviews.scorecard',
    'hrm.offers.view',
    'hrm.referrals.view'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

-- member holds only the scorecard-submit action, so an individual-
-- contributor panelist can score an interview they're on without any
-- broader recruitment visibility.
UPDATE roles
SET permissions = array_cat(permissions, ARRAY['hrm.interviews.scorecard'])
, updated_at = NOW()
WHERE name = 'member' AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY[
        'hrm.interviews.view', 'hrm.interviews.manage', 'hrm.interviews.scorecard',
        'hrm.offers.view', 'hrm.offers.manage',
        'hrm.referrals.view', 'hrm.referrals.manage'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.interviews.view', 'hrm.interviews.manage', 'hrm.interviews.scorecard',
    'hrm.offers.view', 'hrm.offers.manage',
    'hrm.referrals.view', 'hrm.referrals.manage'
);

-- +goose StatementEnd
