-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00133_hrm_payroll_run_uniqueness_per_entity
--
-- Phase 11B-2, discovered while testing: entity-scoped payroll runs were
-- impossible.
--
-- ⚠ uq_hrm_pr_org_month_regular was (org_id, period_year, period_month)
-- WHERE run_type='regular'. That is exactly right for a single-company
-- organization — one regular payroll per month, and the index is what stops
-- somebody paying December twice. But it also means a company operating a
-- German and a British subsidiary CANNOT run their September payrolls
-- separately, which is the entire point of scoping a run to an entity.
--
-- The uniqueness that was actually intended is one regular run per period PER
-- PAYING COMPANY. An org-wide run (legal_entity_id IS NULL) is one such
-- company — the whole organization — so it keeps its own slot.
--
-- ⚠ COALESCE, not a plain four-column index. NULL never equals NULL in a
-- unique index, so (org, year, month, legal_entity_id) would happily allow
-- TWO org-wide runs for the same month and reintroduce the double-payment
-- this index exists to prevent. The sentinel UUID collapses every NULL to one
-- value — the same technique migration 00125 used for
-- uq_hrm_hcsnap_day.
--
-- ⚠ The down block restores the ORIGINAL index definition exactly. Any
-- organization that has created per-entity runs in the same month will fail
-- that restore, which is correct: reverting cannot silently delete a payroll
-- run.
-- ============================================================

DROP INDEX IF EXISTS uq_hrm_pr_org_month_regular;

CREATE UNIQUE INDEX uq_hrm_pr_org_month_regular
    ON hrm_payslip_runs (
        org_id, period_year, period_month,
        COALESCE(legal_entity_id, '00000000-0000-0000-0000-000000000000'::uuid)
    )
    WHERE run_type = 'regular';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS uq_hrm_pr_org_month_regular;

CREATE UNIQUE INDEX uq_hrm_pr_org_month_regular
    ON hrm_payslip_runs (org_id, period_year, period_month)
    WHERE run_type = 'regular';

-- +goose StatementEnd
