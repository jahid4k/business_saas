-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00122_seed_hrm_org_chart_permissions
--
-- Phase 10A permissions. One resource:
--
--   hrm.org_chart — reporting relationships and position seats
--
-- ⚠ NOT SCOPE-TIERED, and that is a decision rather than an omission.
-- scope.Predicate resolves through hrm_employees, so a tiered resource would
-- mean "you may only see the part of the org chart below you" — but an org
-- chart whose shape depends on who is looking is not an org chart, it is a
-- subtree, and every consumer (the chart UI, succession, analytics) needs the
-- whole graph to compute anything. There is therefore no ResolveScope call in
-- this package and TestPermissions_ScopeTiersSeeded does not fire for it.
--
-- The sensitive thing about an org chart is not its SHAPE — who reports to
-- whom is ordinarily public inside a company — it is the salary and appraisal
-- data hanging off each node, and those stay behind their own already-tiered
-- resources.
--
-- .manage is separate from .view because editing a reporting line is not
-- cosmetic: the solid line writes hrm_employees.manager_id, which is what
-- view_team resolves through, so a reporting change silently changes who can
-- see whose payroll. That is an HR-administrative act, not a team-lead one.
--
-- Grant rationale:
--   • owner/admin: view + manage.
--   • manager: view only. A manager needs to see the chart to navigate the
--     org; re-parenting somebody — and thereby moving data access — is not
--     theirs to do.
--   • member/viewer: view. The chart is ordinary internal navigation.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.org_chart.view',   'hrm.org_chart', 'view',   'View the reporting chart and position seats'),
    ('hrm.org_chart.manage', 'hrm.org_chart', 'manage', 'Create and end reporting relationships and manage position seats')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.org_chart.view', 'hrm.org_chart.manage'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.org_chart.view'
]),
updated_at = NOW()
WHERE name IN ('manager', 'member', 'viewer') AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY['hrm.org_chart.view', 'hrm.org_chart.manage'])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN ('hrm.org_chart.view', 'hrm.org_chart.manage');

-- +goose StatementEnd
