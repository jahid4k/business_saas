-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00096_hrm_payroll_engine
--
-- Phase 7A: the payroll engine additions everything else in Phase 7 depends
-- on. No new tables — this widens the two existing ones (migration 00041) and
-- replaces a constraint that made the whole feature impossible.
--
--   hrm_payslip_runs   + run_type, and uq_hrm_pr_org_month REPLACED
--   hrm_payslip_lines  + line_type, is_employer_contribution, source_period_id
--
-- Design notes:
--
--   • THE UNIQUE CONSTRAINT WAS THE BLOCKER. uq_hrm_pr_org_month is
--     UNIQUE (org_id, period_year, period_month) — one run per org per month
--     OF ANY TYPE. Adding run_type without touching it would have produced a
--     column that could never hold more than one value per period: an org
--     could not run a bonus alongside its regular payroll, which is the entire
--     point of the field.
--
--     It is replaced by a PARTIAL unique index over regular runs only. Exactly
--     one regular run per org per month still holds — that invariant is real
--     and payroll depends on it — while off_cycle, bonus, arrears and fnf runs
--     are legitimately repeatable within a period. A leaver's final settlement
--     does not wait for next month.
--
--   • line_type OVERLAPS component_type, AND BOTH ARE NEEDED. component_type
--     ('earning'/'deduction'/'employer_contribution') is the snapshot of what
--     the COMPONENT was; line_type is what the line DOES in the calculation.
--     They diverge as soon as Phase 7 adds lines with no component behind them
--     at all: a loan recovery, a statutory deduction computed from a rule, an
--     arrear recovered from an earlier period. Do not "deduplicate" them.
--
--   • is_employer_contribution is PARTLY derivable from
--     component_type='employer_contribution' — but only partly, which is why
--     it is a real column. Statutory employer contributions (7D) are generated
--     from rules and have no component row, so the fact cannot be recovered
--     from component_type for those lines.
--
--   • source_period_id points at the run whose period an arrear is recovering
--     for. Nullable, because only arrear lines have one.
--
-- Backfill: every existing row predates run_type and line_type, so they are
-- backfilled to 'regular' and to a value derived from component_type. Adding
-- the columns NOT NULL without a backfill would fail on any org with payroll
-- history.
--
-- What must NEVER be added here (the 00076 CHECK × ON DELETE SET NULL trap):
-- Postgres re-evaluates CHECKs on UPDATE and ON DELETE SET NULL *is* an
-- UPDATE, so a CHECK pairing two columns breaks DELETE on the referenced
-- table. Specifically:
--   • CHECK (line_type <> 'arrear' OR source_period_id IS NOT NULL) would make
--     DELETE FROM hrm_payslip_runs fail 23514 for any org holding an arrear
--     line, because source_period_id is ON DELETE SET NULL.
-- The service validates that pairing instead.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_payslip_runs.run_type
-- ------------------------------------------------------------
ALTER TABLE hrm_payslip_runs
    ADD COLUMN run_type TEXT NOT NULL DEFAULT 'regular'
        CHECK (run_type IN ('regular', 'off_cycle', 'bonus', 'arrears', 'fnf'));

COMMENT ON COLUMN hrm_payslip_runs.run_type IS
    'regular is the monthly cycle and is capped at one per org per month by uq_hrm_pr_org_month_regular; the other types are repeatable within a period';

-- Replace the blanket per-month uniqueness with one scoped to regular runs.
ALTER TABLE hrm_payslip_runs DROP CONSTRAINT IF EXISTS uq_hrm_pr_org_month;

CREATE UNIQUE INDEX uq_hrm_pr_org_month_regular
    ON hrm_payslip_runs (org_id, period_year, period_month)
    WHERE run_type = 'regular';

-- Off-cycle runs still need to be distinguishable from one another, so a
-- description is effectively required for them at the service layer. No CHECK
-- here: description is nullable on the table and pairing it with run_type
-- would be the trap described in the header.
CREATE INDEX idx_hrm_pr_org_type ON hrm_payslip_runs (org_id, run_type, period_year, period_month);

-- ------------------------------------------------------------
-- hrm_payslip_lines: line_type, is_employer_contribution, source_period_id
-- ------------------------------------------------------------
ALTER TABLE hrm_payslip_lines
    ADD COLUMN line_type TEXT,
    ADD COLUMN is_employer_contribution BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN source_period_id UUID REFERENCES hrm_payslip_runs(id) ON DELETE SET NULL;

-- Backfill from the existing snapshot before the NOT NULL lands.
UPDATE hrm_payslip_lines
   SET line_type = CASE component_type
                       WHEN 'earning'                 THEN 'earning'
                       WHEN 'deduction'               THEN 'deduction'
                       WHEN 'employer_contribution'   THEN 'earning'
                       ELSE 'earning'
                   END,
       is_employer_contribution = (component_type = 'employer_contribution')
 WHERE line_type IS NULL;

ALTER TABLE hrm_payslip_lines
    ALTER COLUMN line_type SET NOT NULL,
    ALTER COLUMN line_type SET DEFAULT 'earning',
    ADD CONSTRAINT chk_hrm_psl_line_type
        CHECK (line_type IN ('earning', 'deduction', 'arrear',
                             'reimbursement', 'loan_recovery', 'statutory'));

CREATE INDEX idx_hrm_psl_line_type ON hrm_payslip_lines (payslip_id, line_type);
CREATE INDEX idx_hrm_psl_source_period ON hrm_payslip_lines (source_period_id)
    WHERE source_period_id IS NOT NULL;

COMMENT ON COLUMN hrm_payslip_lines.line_type IS
    'What the line DOES in the calculation. Distinct from component_type, which snapshots what the COMPONENT was — lines generated from loans, statutory rules or arrears have no component at all';
COMMENT ON COLUMN hrm_payslip_lines.is_employer_contribution IS
    'Not fully derivable from component_type: statutory employer contributions are generated from rules and carry no component row';
COMMENT ON COLUMN hrm_payslip_lines.source_period_id IS
    'The run whose period an arrear line recovers for; NULL on every other line type';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_hrm_psl_source_period;
DROP INDEX IF EXISTS idx_hrm_psl_line_type;

ALTER TABLE hrm_payslip_lines
    DROP CONSTRAINT IF EXISTS chk_hrm_psl_line_type,
    DROP COLUMN IF EXISTS source_period_id,
    DROP COLUMN IF EXISTS is_employer_contribution,
    DROP COLUMN IF EXISTS line_type;

DROP INDEX IF EXISTS idx_hrm_pr_org_type;
DROP INDEX IF EXISTS uq_hrm_pr_org_month_regular;

ALTER TABLE hrm_payslip_runs DROP COLUMN IF EXISTS run_type;

-- Restore the original blanket constraint. This can only succeed if no org
-- holds two runs in one month — which is exactly the state this migration
-- made reachable, so a rollback after non-regular runs exist will fail loudly
-- rather than silently discarding them.
ALTER TABLE hrm_payslip_runs
    ADD CONSTRAINT uq_hrm_pr_org_month UNIQUE (org_id, period_year, period_month);

-- +goose StatementEnd
