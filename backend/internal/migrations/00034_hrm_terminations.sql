-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00032_hrm_terminations
--
-- HRM employee termination management (Group B4).
--
-- Status machine:
--   draft → pending_approval → approved → applied
--                  ↓                ↓
--               rejected         cancelled
--
-- Termination types:
--   voluntary     → employee chose to leave (use hrm_resignations for formal flow)
--   involuntary   → company-initiated termination
--   layoff        → position eliminated, not performance-based
--   retirement    → age/service-based retirement
--   contract_end  → fixed-term contract natural expiry
--   probation_fail→ failed probation period
--
-- Design notes:
--   • Termination is ALWAYS HR/manager-initiated. Employees use hrm_resignations.
--   • When applied:
--       employee.status            = 'terminated'
--       employee.termination_date  = last_working_date
--   • severance_amount is informational (recorded here, disbursed via payroll/accounting).
--   • is_rehire_eligible default TRUE; set FALSE for gross misconduct cases.
-- ============================================================

CREATE TABLE hrm_terminations (
    id                          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id                   TEXT          NOT NULL UNIQUE
                                                  DEFAULT ('term_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                      UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id                 UUID          NOT NULL REFERENCES hrm_employees(id) ON DELETE RESTRICT,

    termination_type            TEXT          NOT NULL
                                                  CHECK (termination_type IN (
                                                      'voluntary', 'involuntary', 'layoff',
                                                      'retirement', 'contract_end', 'probation_fail'
                                                  )),

    -- Effective dates
    termination_date            DATE          NOT NULL,   -- official termination date
    last_working_date           DATE          NOT NULL,   -- last day in the office

    reason                      TEXT,
    internal_notes              TEXT,         -- private HR notes, not shared with employee

    -- A2: approval chain (e.g. HR + legal must approve involuntary termination)
    approval_instance_id        UUID          REFERENCES hrm_approval_instances(id) ON DELETE SET NULL,

    -- A4: termination letter
    document_id                 UUID          REFERENCES hrm_employee_documents(id) ON DELETE SET NULL,

    -- Severance (informational — disbursed via payroll)
    severance_amount            NUMERIC(15,2),
    severance_currency          TEXT          NOT NULL DEFAULT 'BDT',

    is_rehire_eligible          BOOLEAN       NOT NULL DEFAULT TRUE,
    exit_clearance_completed    BOOLEAN       NOT NULL DEFAULT FALSE,

    status                      TEXT          NOT NULL DEFAULT 'draft'
                                                  CHECK (status IN (
                                                      'draft', 'pending_approval', 'approved',
                                                      'rejected', 'cancelled', 'applied'
                                                  )),

    applied_at                  TIMESTAMPTZ,
    applied_by                  UUID          REFERENCES users(id) ON DELETE SET NULL,

    created_by                  UUID          NOT NULL REFERENCES users(id),
    created_at                  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_term_dates
        CHECK (last_working_date <= termination_date OR last_working_date = termination_date)
);

CREATE INDEX idx_hrm_term_org_id      ON hrm_terminations (org_id);
CREATE INDEX idx_hrm_term_employee_id ON hrm_terminations (employee_id);
CREATE INDEX idx_hrm_term_status      ON hrm_terminations (org_id, status);
CREATE INDEX idx_hrm_term_effective   ON hrm_terminations (termination_date) WHERE status = 'approved';

-- Prevent duplicate active terminations per employee
CREATE UNIQUE INDEX idx_hrm_term_active_employee
    ON hrm_terminations (employee_id)
    WHERE status IN ('draft', 'pending_approval', 'approved');

COMMENT ON TABLE  hrm_terminations IS 'HR-initiated termination records; applies employee.status = terminated';
COMMENT ON COLUMN hrm_terminations.termination_type    IS 'voluntary|involuntary|layoff|retirement|contract_end|probation_fail';
COMMENT ON COLUMN hrm_terminations.internal_notes      IS 'Private HR notes — not exposed to the employee';
COMMENT ON COLUMN hrm_terminations.severance_amount    IS 'Informational; disbursed via payroll/accounting module';
COMMENT ON COLUMN hrm_terminations.is_rehire_eligible  IS 'FALSE for gross misconduct; blocks future rehire in HR tools';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_terminations;

-- +goose StatementEnd
