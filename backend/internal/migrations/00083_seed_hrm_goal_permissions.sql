-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00083_seed_hrm_goal_permissions
--
-- Phase 5A permissions. Two resources:
--
--   hrm.goals        — the goals themselves, scope-tiered
--   hrm.goal_cycles  — org-level period configuration, NOT scope-tiered
--
-- ⚠ The three scope tiers are MANDATORY, not optional. Unlike recruitment
-- (see 00079's header), goals rows carry an employee_id, so they qualify for
-- the Phase 1 tiers — and the build plan names draft leakage as this module's
-- failure mode. TestPermissions_ScopeTiersSeeded finds every ResolveScope()
-- call and requires ALL THREE of <resource>.view_own / .view_team / .view_all
-- to be seeded, all-or-nothing. Omitting one does not degrade gracefully:
-- callers holding the missing tier resolve to ScopeNone and silently see
-- nothing, which is why the test exists.
--
-- Conversely, do NOT call ResolveScope(..., "hrm.goal_cycles"). Cycles are
-- org-level rows with no employee_id for scope.Predicate to filter on; doing
-- so would force three more seeded keys and imply a filter that cannot exist.
--
-- Two grant decisions worth recording:
--
--   • hrm.goals.set_own is granted through 'member' and gates every goal
--     WRITE route, then the service narrows it: writing your own goal needs
--     only set_own, while writing someone else's additionally requires
--     hrm.goals.manage AND passing scope.AuthorizeRecordAccess. The route gate
--     cannot express "is this your own goal", so it does not try — the
--     platform.checklists.complete (00077) and hrm.interviews.scorecard
--     (00081) precedent.
--
--   • hrm.goals.manage therefore NEVER appears in a permFn(...) call. It is
--     checked in the handler via authzSvc.Can(...) and passed to the service
--     on a Caller struct. No architecture test requires a seeded key to appear
--     in a route registration; TestPermissions_UsedStringsExistInMigrations
--     only enforces the reverse direction.
--
-- There is deliberately no hrm.goals.checkin key. 00081 justifies splitting a
-- write action into its own key only when some role needs one without the
-- other, and no role here does — checking in is a write on a goal, gated by
-- set_own and narrowed identically. 5C's 360 feedback is where a genuine
-- "write on someone else's record" key belongs.
--
-- 'member' holds goal_cycles.view because creating a goal requires picking a
-- cycle. As with 00079/00081, 'viewer' gets nothing and org-created custom
-- roles get nothing until an admin grants it explicitly.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.goals.view',         'hrm.goals',       'view',      'View goals'),
    ('hrm.goals.manage',       'hrm.goals',       'manage',    'Create and administer goals belonging to other employees'),
    ('hrm.goals.set_own',      'hrm.goals',       'set_own',   'Create, update and check in on goals'),
    ('hrm.goals.view_own',     'hrm.goals',       'view_own',  'View own goals only'),
    ('hrm.goals.view_team',    'hrm.goals',       'view_team', 'View own and direct reports'' goals'),
    ('hrm.goals.view_all',     'hrm.goals',       'view_all',  'View all goals in the organization'),
    ('hrm.goal_cycles.view',   'hrm.goal_cycles', 'view',      'View goal cycles'),
    ('hrm.goal_cycles.manage', 'hrm.goal_cycles', 'manage',    'Create, activate, lock and close goal cycles')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.goals.view', 'hrm.goals.manage', 'hrm.goals.set_own', 'hrm.goals.view_all',
    'hrm.goal_cycles.view', 'hrm.goal_cycles.manage'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.goals.view', 'hrm.goals.manage', 'hrm.goals.set_own', 'hrm.goals.view_team',
    'hrm.goal_cycles.view'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

-- member can set and check in on their OWN goals (the service narrows
-- set_own to self), and sees only their own.
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.goals.view', 'hrm.goals.set_own', 'hrm.goals.view_own',
    'hrm.goal_cycles.view'
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
        'hrm.goals.view', 'hrm.goals.manage', 'hrm.goals.set_own',
        'hrm.goals.view_own', 'hrm.goals.view_team', 'hrm.goals.view_all',
        'hrm.goal_cycles.view', 'hrm.goal_cycles.manage'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.goals.view', 'hrm.goals.manage', 'hrm.goals.set_own',
    'hrm.goals.view_own', 'hrm.goals.view_team', 'hrm.goals.view_all',
    'hrm.goal_cycles.view', 'hrm.goal_cycles.manage'
);

-- +goose StatementEnd
