-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00097_seed_hrm_payroll_preview_permission
--
-- Phase 7A permissions. One new key on the existing hrm.payroll resource:
--
--   hrm.payroll.preview — compute a run WITHOUT persisting anything.
--
-- The build plan requires a "mandatory dry-run preview before finalize", and
-- nothing of the sort existed: the only way to see what payroll would produce
-- was to run ComputeRun, which writes payslips.
--
-- Why its own key rather than reusing .compute. Preview is READ-SHAPED — it
-- persists nothing — so it is safe to grant far more widely than the action
-- that writes everyone's payslips. Reusing .compute would have meant anyone
-- allowed to sanity-check the numbers was also allowed to commit them, which
-- inverts the point of a dry run.
--
-- Grants: owner/admin get it with the rest of payroll. 'manager' gets it too,
-- and that is deliberate — a manager already holds hrm.payroll.view, so
-- preview discloses nothing they cannot already see, and letting them check a
-- period before HR commits it is exactly the review the build plan wants.
-- 'member' does not: a member sees only their own payslips, and a preview is
-- org-wide.
--
-- hrm.payroll IS already scope-tiered — view_own/view_team/view_all were
-- seeded by migration 00072, ResolveScope is called in payslips/handler.go,
-- and the payslip list applies scope.Predicate on employee_id. No new tier
-- keys are needed here and none may be removed.
--
-- Note the asymmetry, because it is easy to misread: the TIERS govern which
-- PAYSLIPS a caller sees (payslips carry an employee_id). RUNS are org-level
-- objects with no employee_id, so .preview is an untiered org-wide capability
-- — which is exactly why it is granted narrowly rather than to 'member'.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.payroll.preview', 'hrm.payroll', 'preview',
     'Dry-run a payroll computation without persisting payslips')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY['hrm.payroll.preview']),
    updated_at = NOW()
WHERE name IN ('owner', 'admin', 'manager') AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions) EXCEPT SELECT unnest(ARRAY['hrm.payroll.preview'])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key = 'hrm.payroll.preview';

-- +goose StatementEnd
