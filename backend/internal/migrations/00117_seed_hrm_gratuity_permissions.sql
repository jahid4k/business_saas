-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00117_seed_hrm_gratuity_permissions
--
-- Phase 9B permissions. One new resource:
--
--   hrm.gratuity — the effective-dated gratuity rule table
--
-- ⚠ NOT scope-tiered. hrm_gratuity_rules is org-level CATALOGUE data with no
-- employee_id, and scope.Predicate resolves through hrm_employees — so there
-- is no ResolveScope call for it and TestPermissions_ScopeTiersSeeded does
-- not fire. Same split as hrm.asset_config (8A) and hrm.expense_config (8B).
--
-- ⚠ F&F APPROVAL NEEDS NO NEW KEY. hrm.exits.settle was already seeded by
-- 00115, deliberately ahead of its consumer, because
-- TestPermissions_ScopeTiersSeeded is all-or-nothing per resource and every
-- hrm.exits.* key had to ship together. This slice is where it starts being
-- enforced — separation of duties: running clearance is HR-operational work,
-- approving the money that leaves with the employee is a finance authority.
--
-- Grant rationale:
--   • owner/admin: view + manage. Gratuity terms are a policy decision with
--     direct payroll consequence.
--   • manager: view only. A department head should be able to explain a
--     leaver's entitlement without being able to change what it is.
--   • member/viewer: nothing. An employee's own gratuity reaches them on
--     their payslip, which they already read through hrm.payroll.view_own —
--     the rule table itself is org policy, not personal data.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.gratuity.view',   'hrm.gratuity', 'view',   'View gratuity rules'),
    ('hrm.gratuity.manage', 'hrm.gratuity', 'manage', 'Create and revise effective-dated gratuity rules')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.gratuity.view', 'hrm.gratuity.manage'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.gratuity.view'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY['hrm.gratuity.view', 'hrm.gratuity.manage'])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN ('hrm.gratuity.view', 'hrm.gratuity.manage');

-- +goose StatementEnd
