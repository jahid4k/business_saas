-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00027_hrm_contracts
--
-- HRM employee contract management (Group A7):
--   hrm_employee_contracts — employment contract records
--
-- Design notes:
--   • contract_type drives which date fields are meaningful:
--       permanent  → no end_date; probation_end_date common
--       fixed_term → end_date required
--       probation  → probation_end_date required (short-term trial)
--       internship → end_date required
--       consultant → end_date optional; hourly/project basis
--   • notice_period_days — used by B3 (Resignation) to auto-compute last day.
--   • salary_structure_id links to A1 — contract implicitly sets salary grade.
--   • document_id links to A4 — the signed contract file.
--   • Only one active contract per employee at a time (enforced by partial unique index).
--   • When a new contract is created, the previous one should be set is_active = FALSE.
--     The service layer handles this transition — not a DB trigger.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_employee_contracts
-- ------------------------------------------------------------
CREATE TABLE hrm_employee_contracts (
    id                    UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id             TEXT    NOT NULL UNIQUE
                                      DEFAULT ('ec_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                UUID    NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id           UUID    NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    contract_type         TEXT    NOT NULL
                                      CHECK (contract_type IN (
                                          'permanent', 'fixed_term', 'probation',
                                          'internship', 'consultant'
                                      )),

    -- Date range
    start_date            DATE    NOT NULL,
    end_date              DATE,               -- NULL for permanent/open-ended
    probation_end_date    DATE,               -- NULL if no probation period

    -- Notice period (used by B3 Resignation)
    notice_period_days    INTEGER NOT NULL DEFAULT 30 CHECK (notice_period_days >= 0),

    -- Compensation link (optional — can exist without a salary structure)
    salary_structure_id   UUID    REFERENCES hrm_salary_structures(id) ON DELETE SET NULL,
    work_hours_per_week   NUMERIC(5,2),

    -- Signed contract document (A4)
    document_id           UUID    REFERENCES hrm_employee_documents(id) ON DELETE SET NULL,

    -- Status
    is_active             BOOLEAN NOT NULL DEFAULT TRUE,

    notes                 TEXT,

    created_by            UUID    NOT NULL REFERENCES users(id),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_ec_dates
        CHECK (end_date IS NULL OR end_date > start_date),
    CONSTRAINT chk_hrm_ec_probation
        CHECK (probation_end_date IS NULL OR probation_end_date >= start_date)
);

CREATE INDEX idx_hrm_ec_org_id      ON hrm_employee_contracts (org_id);
CREATE INDEX idx_hrm_ec_employee_id ON hrm_employee_contracts (employee_id);
CREATE INDEX idx_hrm_ec_end_date    ON hrm_employee_contracts (end_date)
    WHERE end_date IS NOT NULL;

-- Only one active contract per employee
CREATE UNIQUE INDEX idx_hrm_ec_active
    ON hrm_employee_contracts (employee_id)
    WHERE is_active = TRUE;

COMMENT ON TABLE  hrm_employee_contracts IS 'Employee contract records; one active per employee at a time';
COMMENT ON COLUMN hrm_employee_contracts.notice_period_days IS 'Used by resignation flow to auto-compute last working day';
COMMENT ON COLUMN hrm_employee_contracts.is_active          IS 'Only one row may have is_active=TRUE per employee (partial unique index)';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_employee_contracts;

-- +goose StatementEnd
