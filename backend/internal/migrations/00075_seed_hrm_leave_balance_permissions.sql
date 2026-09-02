-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00075_seed_hrm_leave_balance_permissions
--
-- HRM Phase 2 (Leave Engine Upgrade): two new action keys on the existing
-- hrm.leave resource for the two balance-correction actions that need
-- gating distinct from the routine hrm.leave.approve permission:
--   hrm.leave.adjust_balance — manual HR correction to an employee's balance
--   hrm.leave.encash         — process a leave encashment (records days only,
--                              per Phase 2 scope — no money computed)
--
-- Granted to owner/admin only, not manager — matches the existing
-- hrm.salary.employee.manage precedent (migration 00030), which draws the
-- same line between routine approval-level trust and financial-correction-
-- level trust.
--
-- No new view_own/view_team/view_all seeding needed: balance/ledger reads
-- reuse the hrm.leave.view_own/view_team/view_all tiers already seeded in
-- migration 00072, since authz.Service.ResolveScope keys purely on the
-- "hrm.leave" resource string, independent of which route/action call it.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.leave.adjust_balance', 'hrm.leave', 'adjust_balance', 'Manually correct an employee leave balance'),
    ('hrm.leave.encash',         'hrm.leave', 'encash',         'Process a leave encashment')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.leave.adjust_balance', 'hrm.leave.encash'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY['hrm.leave.adjust_balance', 'hrm.leave.encash'])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN ('hrm.leave.adjust_balance', 'hrm.leave.encash');

-- +goose StatementEnd
