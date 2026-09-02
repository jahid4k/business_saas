-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00115_seed_hrm_exit_permissions
--
-- Phase 9A permissions. Two resources:
--
--   hrm.exits        — the exit record itself, SCOPE-TIERED
--   hrm.exit_config  — clearance templates and rehire decisions, untiered
--
-- ⚠ hrm.exits IS SCOPE-TIERED and hrm.exit_config IS NOT, and the split is
-- the same one 8A drew for hrm.assets vs hrm.asset_config: scope.Predicate
-- resolves through hrm_employees, so only tables carrying an employee_id
-- qualify. hrm_exits does; clearance configuration does not.
--
-- TestPermissions_ScopeTiersSeeded is ALL-OR-NOTHING per resource, so
-- hrm.exits ships view_own/view_team/view_all together or the guard fails.
--
-- .view_own is meaningful here and not merely mechanical: a departing
-- employee has a legitimate need to watch their own clearance and see what
-- is holding up their settlement, which is exactly the frustration this
-- module exists to remove.
--
-- .manage is separate from .settle deliberately. Initiating an exit and
-- running clearance is HR-operational work; approving the money that leaves
-- with the employee is a finance authority. An org where the person who
-- processes leavers can also sign off their own final payment has no
-- separation of duties at all. Phase 9B gates F&F approval on .settle.
--
-- .decide_rehire is its own key because it is the one decision here that
-- outlives the exit and follows a person into a future hiring process — a
-- wrong "not eligible" is quietly expensive and should not be a side effect
-- of holding general exit-management rights.
--
-- Grant rationale:
--   • owner/admin: everything.
--   • manager: view_team + view_own, and clearance resolution — a department
--     head clears their own outstanding items. NOT .settle, NOT
--     .decide_rehire, NOT view_all.
--   • member: view_own only. Their own exit, nothing else.
--   • viewer: nothing. An exit record carries settlement figures and
--     rehire decisions; a read-only role has no business in it.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.exits.view_own',      'hrm.exits', 'view_own',      'View your own exit record and clearance'),
    ('hrm.exits.view_team',     'hrm.exits', 'view_team',     'View exit records for your reporting line'),
    ('hrm.exits.view_all',      'hrm.exits', 'view_all',      'View every exit record in the organization'),
    ('hrm.exits.manage',        'hrm.exits', 'manage',        'Initiate, update and cancel exit records'),
    ('hrm.exits.clear',         'hrm.exits', 'clear',         'Resolve clearance items and record outstanding amounts'),
    ('hrm.exits.settle',        'hrm.exits', 'settle',        'Approve the full and final settlement'),
    ('hrm.exits.decide_rehire', 'hrm.exits', 'decide_rehire', 'Record whether a former employee may be rehired'),

    ('hrm.exit_config.view',    'hrm.exit_config', 'view',    'View exit clearance configuration'),
    ('hrm.exit_config.manage',  'hrm.exit_config', 'manage',  'Administer exit clearance configuration')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.exits.view_own', 'hrm.exits.view_team', 'hrm.exits.view_all',
    'hrm.exits.manage', 'hrm.exits.clear', 'hrm.exits.settle', 'hrm.exits.decide_rehire',
    'hrm.exit_config.view', 'hrm.exit_config.manage'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.exits.view_own', 'hrm.exits.view_team', 'hrm.exits.clear',
    'hrm.exit_config.view'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.exits.view_own'
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
        'hrm.exits.view_own', 'hrm.exits.view_team', 'hrm.exits.view_all',
        'hrm.exits.manage', 'hrm.exits.clear', 'hrm.exits.settle', 'hrm.exits.decide_rehire',
        'hrm.exit_config.view', 'hrm.exit_config.manage'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.exits.view_own', 'hrm.exits.view_team', 'hrm.exits.view_all',
    'hrm.exits.manage', 'hrm.exits.clear', 'hrm.exits.settle', 'hrm.exits.decide_rehire',
    'hrm.exit_config.view', 'hrm.exit_config.manage'
);

-- +goose StatementEnd
