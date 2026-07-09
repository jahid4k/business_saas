-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00039_hrm_payslips
--
-- HRM payslip engine — Group D2.
-- Depends on:
--   A1: hrm_salary_structures, hrm_salary_components, hrm_salary_structure_components,
--       hrm_employee_salary_records (basic_pay)
--   D1: hrm_attendance_periods (must be 'finalized' before payroll can run)
--
-- Three tables:
--   hrm_payslip_runs   — one batch per org per month (the payroll run)
--   hrm_payslips       — one row per employee per run
--   hrm_payslip_lines  — one row per salary component per payslip
--
-- Status machines:
--   payslip_run: draft → computing → computed → approved → paid | cancelled
--   payslip:     draft → computed → approved → paid
--
-- Formula engine (A1 integration):
--   Components with calc_method='formula' are evaluated using
--   github.com/expr-lang/expr with env:
--     BASIC           = employee's basic_pay (from hrm_employee_salary_records)
--     GROSS           = rolling sum of earnings computed so far
--     PRESENT_DAYS    = attendance days present this month
--     WORK_DAYS       = org working days in the month
--     TENURE_YEARS    = years since hire_date (float64)
--
--   Evaluation order: components ordered by display_order (hrm_salary_structure_components).
--   GROSS accumulates as each earning is computed; deductions see the final gross.
--
-- D1 ↔ D2 hard dependency:
--   Service.ComputeRun() checks attendance_period.status='finalized' before proceeding.
--   If attendance_period_id is NULL, compute proceeds without attendance data
--   (attendance summary fields are zero — for orgs that don't use D1).
-- ============================================================

-- ------------------------------------------------------------
-- hrm_payslip_runs
-- ------------------------------------------------------------
CREATE TABLE hrm_payslip_runs (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id            TEXT        NOT NULL UNIQUE
                                         DEFAULT ('pr_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id               UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    period_year          INTEGER     NOT NULL CHECK (period_year >= 2020),
    period_month         INTEGER     NOT NULL CHECK (period_month BETWEEN 1 AND 12),

    description          TEXT,
    currency             TEXT        NOT NULL DEFAULT 'BDT',

    -- D1 link (optional — orgs not using D1 may omit)
    attendance_period_id UUID        REFERENCES hrm_attendance_periods(id) ON DELETE SET NULL,

    -- Aggregate stats (populated at compute time)
    total_employees      INTEGER     NOT NULL DEFAULT 0,
    total_gross_pay      NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_deductions     NUMERIC(18,2) NOT NULL DEFAULT 0,
    total_net_pay        NUMERIC(18,2) NOT NULL DEFAULT 0,

    status               TEXT        NOT NULL DEFAULT 'draft'
                                         CHECK (status IN (
                                             'draft', 'computing', 'computed',
                                             'approved', 'paid', 'cancelled'
                                         )),

    computed_at          TIMESTAMPTZ,
    computed_by          UUID        REFERENCES users(id) ON DELETE SET NULL,
    approved_at          TIMESTAMPTZ,
    approved_by          UUID        REFERENCES users(id) ON DELETE SET NULL,
    paid_at              TIMESTAMPTZ,
    paid_by              UUID        REFERENCES users(id) ON DELETE SET NULL,

    created_by           UUID        NOT NULL REFERENCES users(id),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One payroll run per org per month
    CONSTRAINT uq_hrm_pr_org_month UNIQUE (org_id, period_year, period_month)
);

CREATE INDEX idx_hrm_pr_org_id ON hrm_payslip_runs (org_id);
CREATE INDEX idx_hrm_pr_status ON hrm_payslip_runs (org_id, status);

COMMENT ON TABLE  hrm_payslip_runs IS 'Monthly payroll run batch; one per org per month';
COMMENT ON COLUMN hrm_payslip_runs.attendance_period_id IS 'Links to D1 finalized period; NULL = org not using attendance module';
COMMENT ON COLUMN hrm_payslip_runs.status               IS 'computing=run in progress (prevents duplicate concurrent runs)';

-- ------------------------------------------------------------
-- hrm_payslips
-- ------------------------------------------------------------
CREATE TABLE hrm_payslips (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id            TEXT        NOT NULL UNIQUE
                                         DEFAULT ('ps_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id               UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id          UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE RESTRICT,
    payslip_run_id       UUID        NOT NULL REFERENCES hrm_payslip_runs(id) ON DELETE CASCADE,

    period_year          INTEGER     NOT NULL,
    period_month         INTEGER     NOT NULL,

    -- Salary structure reference (A1) — snapshot at compute time
    salary_structure_id  UUID        REFERENCES hrm_salary_structures(id) ON DELETE SET NULL,
    salary_structure_name TEXT,       -- snapshot

    -- Financial summary (populated at compute time, derived from lines)
    gross_pay            NUMERIC(15,2) NOT NULL DEFAULT 0,
    total_deductions     NUMERIC(15,2) NOT NULL DEFAULT 0,
    net_pay              NUMERIC(15,2) NOT NULL DEFAULT 0,
    basic_pay            NUMERIC(15,2) NOT NULL DEFAULT 0,  -- snapshot from hrm_employee_salary_records

    -- Attendance summary for this employee this month (from D1, if available)
    work_days            INTEGER     NOT NULL DEFAULT 0,
    present_days         INTEGER     NOT NULL DEFAULT 0,
    absent_days          INTEGER     NOT NULL DEFAULT 0,
    leave_days           INTEGER     NOT NULL DEFAULT 0,
    holiday_days         INTEGER     NOT NULL DEFAULT 0,
    overtime_hours       NUMERIC(7,2) NOT NULL DEFAULT 0,

    currency             TEXT        NOT NULL DEFAULT 'BDT',

    status               TEXT        NOT NULL DEFAULT 'draft'
                                         CHECK (status IN ('draft', 'computed', 'approved', 'paid')),

    -- Payment tracking
    payment_reference    TEXT,
    payment_date         DATE,
    paid_at              TIMESTAMPTZ,

    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One payslip per employee per run
    CONSTRAINT uq_hrm_ps_run_employee UNIQUE (payslip_run_id, employee_id)
);

CREATE INDEX idx_hrm_ps_org_id      ON hrm_payslips (org_id);
CREATE INDEX idx_hrm_ps_employee_id ON hrm_payslips (employee_id);
CREATE INDEX idx_hrm_ps_run_id      ON hrm_payslips (payslip_run_id);
CREATE INDEX idx_hrm_ps_period      ON hrm_payslips (org_id, period_year, period_month);

COMMENT ON TABLE  hrm_payslips IS 'Per-employee payslip; one per run per employee';
COMMENT ON COLUMN hrm_payslips.basic_pay             IS 'Snapshot from hrm_employee_salary_records at compute time; formula env BASIC variable';
COMMENT ON COLUMN hrm_payslips.salary_structure_name IS 'Snapshot at compute time; immune to structure rename';

-- ------------------------------------------------------------
-- hrm_payslip_lines
-- ------------------------------------------------------------
CREATE TABLE hrm_payslip_lines (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    payslip_id           UUID        NOT NULL REFERENCES hrm_payslips(id) ON DELETE CASCADE,
    org_id               UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Component reference (A1) — may be NULL if component deleted after payslip computed
    component_id         UUID        REFERENCES hrm_salary_components(id) ON DELETE SET NULL,
    component_name       TEXT        NOT NULL,   -- snapshot at compute time
    component_type       TEXT        NOT NULL    -- earning|deduction|employer_contribution (snapshot)
                                         CHECK (component_type IN ('earning', 'deduction', 'employer_contribution')),

    -- How the amount was computed
    calc_method          TEXT        NOT NULL,   -- snapshot: fixed|pct_of_basic|pct_of_gross|formula|manual|slab
    formula_used         TEXT,                   -- snapshot of formula_expression (for audit)
    computed_amount      NUMERIC(15,2) NOT NULL DEFAULT 0,

    -- Display order (from hrm_salary_structure_components.display_order)
    display_order        INTEGER     NOT NULL DEFAULT 0,

    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_pl_payslip_id ON hrm_payslip_lines (payslip_id);
CREATE INDEX idx_hrm_pl_org_id     ON hrm_payslip_lines (org_id);
CREATE INDEX idx_hrm_pl_order      ON hrm_payslip_lines (payslip_id, display_order);

COMMENT ON TABLE  hrm_payslip_lines IS 'Salary component breakdown per payslip; one row per component';
COMMENT ON COLUMN hrm_payslip_lines.formula_used    IS 'Snapshot of formula_expression at compute time for audit trail';
COMMENT ON COLUMN hrm_payslip_lines.computed_amount IS 'Final computed value; always positive (sign determined by component_type)';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_payslip_lines;
DROP TABLE IF EXISTS hrm_payslips;
DROP TABLE IF EXISTS hrm_payslip_runs;

-- +goose StatementEnd
