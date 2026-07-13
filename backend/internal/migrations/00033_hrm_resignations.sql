-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00031_hrm_resignations
--
-- HRM employee resignation management (Group B3).
--
-- Status machine:
--   submitted → accepted → (employee status updated on last_working_date)
--      ↓            ↓
--   rejected   withdrawn
--
-- Design notes:
--   • This migration also extends hrm_employees.status to include 'resigned'.
--     When HR accepts a resignation, the employee status becomes 'resigned'
--     (active payroll continues until last_working_date; status = 'terminated'
--      after the employee officially leaves).
--
--   • notice_period_days is a SNAPSHOT from the active contract (A7) at
--     submission time. Contract changes after submission don't retroactively
--     change the computed last_working_date.
--
--   • last_working_date = resignation_date + notice_period_days calendar days.
--     HR may override this (is_notice_waived = TRUE → HR sets last_working_date directly).
--
--   • Constraint: only one active resignation per employee (submitted or accepted).
--     Enforced by partial unique index.
-- ============================================================

-- Extend employee status to include 'resigned'
ALTER TABLE hrm_employees
    DROP CONSTRAINT IF EXISTS hrm_employees_status_check,
    ADD CONSTRAINT hrm_employees_status_check
        CHECK (status IN ('active', 'inactive', 'on_leave', 'terminated', 'resigned'));

COMMENT ON COLUMN hrm_employees.status IS 'active | inactive | on_leave | resigned | terminated';

-- ------------------------------------------------------------
-- hrm_resignations
-- ------------------------------------------------------------
CREATE TABLE hrm_resignations (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id                   TEXT        NOT NULL UNIQUE
                                                DEFAULT ('res_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id                 UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE RESTRICT,

    -- When the resignation was submitted (usually today, but allow backdating)
    resignation_date            DATE        NOT NULL DEFAULT CURRENT_DATE,

    -- Notice period (snapshotted from hrm_employee_contracts at submission time)
    notice_period_days          INTEGER     NOT NULL DEFAULT 30 CHECK (notice_period_days >= 0),
    is_notice_waived            BOOLEAN     NOT NULL DEFAULT FALSE,

    -- Computed: resignation_date + notice_period_days (or HR override if waived)
    last_working_date           DATE        NOT NULL,

    -- Reason
    reason_category             TEXT        NOT NULL DEFAULT 'other'
                                                CHECK (reason_category IN (
                                                    'personal', 'career_growth',
                                                    'better_opportunity', 'relocation',
                                                    'health', 'retirement', 'other'
                                                )),
    reason_remarks              TEXT,

    -- A2: approval/acknowledgement chain (manager + HR must acknowledge)
    approval_instance_id        UUID        REFERENCES hrm_approval_instances(id) ON DELETE SET NULL,

    -- A4: resignation acceptance letter
    document_id                 UUID        REFERENCES hrm_employee_documents(id) ON DELETE SET NULL,

    -- Exit process
    exit_interview_completed    BOOLEAN     NOT NULL DEFAULT FALSE,
    exit_clearance_completed    BOOLEAN     NOT NULL DEFAULT FALSE,

    status                      TEXT        NOT NULL DEFAULT 'submitted'
                                                CHECK (status IN ('submitted', 'accepted', 'withdrawn', 'rejected')),

    accepted_at                 TIMESTAMPTZ,
    accepted_by                 UUID        REFERENCES users(id) ON DELETE SET NULL,

    created_by                  UUID        NOT NULL REFERENCES users(id),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_res_dates
        CHECK (last_working_date >= resignation_date)
);

CREATE INDEX idx_hrm_res_org_id      ON hrm_resignations (org_id);
CREATE INDEX idx_hrm_res_employee_id ON hrm_resignations (employee_id);
CREATE INDEX idx_hrm_res_status      ON hrm_resignations (org_id, status);

-- Only one active resignation per employee at a time
CREATE UNIQUE INDEX idx_hrm_res_active_employee
    ON hrm_resignations (employee_id)
    WHERE status IN ('submitted', 'accepted');

COMMENT ON TABLE  hrm_resignations IS 'Employee resignation records; one active per employee at a time';
COMMENT ON COLUMN hrm_resignations.notice_period_days IS 'Snapshot from active contract at submission time';
COMMENT ON COLUMN hrm_resignations.last_working_date  IS 'resignation_date + notice_period_days; overridden if notice is waived';
COMMENT ON COLUMN hrm_resignations.is_notice_waived   IS 'When true, HR sets last_working_date directly ignoring notice_period_days';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_resignations;

-- Revert employee status constraint (removes 'resigned')
ALTER TABLE hrm_employees
    DROP CONSTRAINT IF EXISTS hrm_employees_status_check,
    ADD CONSTRAINT hrm_employees_status_check
        CHECK (status IN ('active', 'inactive', 'on_leave', 'terminated'));

-- +goose StatementEnd
