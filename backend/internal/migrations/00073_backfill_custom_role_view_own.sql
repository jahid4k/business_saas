-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00073_backfill_custom_role_view_own
--
-- Phase 1 backward-compatibility fix. 00072 seeded view_own/view_team/
-- view_all for the 12 HRM record-level modules and granted them to the five
-- SYSTEM roles (org_id IS NULL). It did NOT touch org-created custom roles
-- or per-member custom_permissions overrides.
--
-- Without this migration, any custom role or member override holding a bare
-- hrm.<resource>.view but none of the three new tier keys would, once the
-- application code starts calling authzSvc.ResolveScope, pass the unchanged
-- route-level gate but resolve to ScopeNone — silently returning an empty
-- list (200 OK), not an error. That's a real regression, not a tightening:
-- nobody reviewed those custom grants with the new tiering in mind.
--
-- This backfill grants view_own (never view_all — that would ship zero
-- actual tightening for any org that customized a role) as a floor to every
-- custom role / member override currently holding bare hrm.<resource>.view,
-- for every resource EXCEPT hrm.terminations — employees have never had
-- self-view of termination records (see 00072's header and
-- internal/hrm/terminations/routes.go's existing design comment); this
-- backfill must not silently introduce that capability for a custom role
-- either.
--
-- One-way forward migration: distinguishing "backfill-added" from
-- "admin-added post-deploy" view_own grants after the fact isn't reliable,
-- so there is no precise down-migration — matches this codebase's existing
-- precedent for permission-seed migrations.
--
-- Before rolling this out to a production database, run the read-only audit
-- query below and hand the results to customer success so any org whose
-- custom role is affected can be told proactively:
--
--   SELECT org_id, name, permissions FROM roles
--   WHERE org_id IS NOT NULL
--     AND permissions && ARRAY[
--       'hrm.employees.view','hrm.leave.view','hrm.salary.employee.view','hrm.payroll.view',
--       'hrm.attendance.view','hrm.warnings.view','hrm.complaints.view','hrm.documents.view',
--       'hrm.promotions.view','hrm.transfers.view','hrm.resignations.view'
--     ];
-- ============================================================

DO $$
DECLARE
    res TEXT;
    resources TEXT[] := ARRAY[
        'hrm.employees', 'hrm.leave', 'hrm.salary.employee', 'hrm.payroll',
        'hrm.attendance', 'hrm.warnings', 'hrm.complaints', 'hrm.documents',
        'hrm.promotions', 'hrm.transfers', 'hrm.resignations'
        -- hrm.terminations intentionally excluded — see header.
    ];
BEGIN
    FOREACH res IN ARRAY resources LOOP
        UPDATE roles
        SET permissions = array_append(permissions, res || '.view_own'), updated_at = NOW()
        WHERE org_id IS NOT NULL
          AND (res || '.view') = ANY(permissions)
          AND NOT (permissions && ARRAY[res || '.view_own', res || '.view_team', res || '.view_all']);

        UPDATE organization_members
        SET custom_permissions = array_append(custom_permissions, res || '.view_own'), updated_at = NOW()
        WHERE (res || '.view') = ANY(custom_permissions)
          AND NOT (custom_permissions && ARRAY[res || '.view_own', res || '.view_team', res || '.view_all']);
    END LOOP;
END $$;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- No precise down-migration — see header. Down is a deliberate no-op; roll
-- back 00072 (which removes the underlying permission keys entirely) if the
-- whole tiering feature needs to be reverted.

-- +goose StatementEnd
