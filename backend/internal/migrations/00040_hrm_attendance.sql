-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00038_hrm_attendance
--
-- HRM attendance tracking — Group D1.
-- Depends on: A5 (hrm_shifts), A6 (hrm_holiday_calendars), A2 (hrm_approval_instances)
--
-- Two tables:
--   hrm_attendance_records  — one row per employee per day
--   hrm_attendance_periods  — monthly lock/finalize for payroll handoff
--
-- Shift resolution (service layer, not schema):
--   Look up hrm_work_schedule_assignments with priority:
--   employee-level > department-level > org-level > default shift.
--   Snapshot shift fields (shift_id, expected_in, expected_out) at record time
--   so retroactive shift changes don't corrupt historical attendance.
--
-- regular_hours / overtime_hours:
--   Computed at record time from check_in/check_out + break_minutes.
--   OT applies only when shift.track_overtime = TRUE and
--   duration > shift.overtime_threshold_hours.
--
-- status machine (regularization):
--   pending → approved (manager/HR approves the record)
--            → rejected (if regularization is denied)
--   Normal auto-approved records start as 'approved'.
--   Regularization requests require HR review → 'pending'.
--
-- D1 ↔ D2 handoff:
--   When a period is finalized, hrm_attendance_periods.status = 'finalized'.
--   The payslip compute engine (D2) checks this before running.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_attendance_records
-- ------------------------------------------------------------
CREATE TABLE hrm_attendance_records (
    id                         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id                  TEXT        NOT NULL UNIQUE
                                               DEFAULT ('att_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                     UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id                UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE RESTRICT,

    attendance_date            DATE        NOT NULL,

    -- Shift snapshot at record creation (lookup via hrm_work_schedule_assignments)
    shift_id                   UUID        REFERENCES hrm_shifts(id) ON DELETE SET NULL,
    shift_name                 TEXT,                -- snapshot: immune to shift rename
    expected_in                TIME,                -- snapshot: shift start_time
    expected_out               TIME,                -- snapshot: shift end_time

    -- Actual times (NULL if absent / holiday / weekend)
    check_in_time              TIME,
    check_out_time             TIME,
    break_minutes              INTEGER     NOT NULL DEFAULT 0 CHECK (break_minutes >= 0),

    -- Computed on save (service layer)
    regular_hours              NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (regular_hours >= 0),
    overtime_hours             NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (overtime_hours >= 0),

    -- Day classification
    day_type                   TEXT        NOT NULL DEFAULT 'present'
                                               CHECK (day_type IN (
                                                   'present', 'absent', 'half_day', 'late',
                                                   'on_leave', 'holiday', 'weekend', 'work_from_home'
                                               )),

    -- How the record was created
    source                     TEXT        NOT NULL DEFAULT 'manual'
                                               CHECK (source IN ('manual', 'device', 'api', 'system')),

    notes                      TEXT,

    -- Regularization (employee requests a correction on an already-approved record)
    regularization_reason      TEXT,
    regularization_instance_id UUID        REFERENCES hrm_approval_instances(id) ON DELETE SET NULL,

    -- Approval state (auto-approved for system records; pending for regularizations)
    status                     TEXT        NOT NULL DEFAULT 'approved'
                                               CHECK (status IN ('pending', 'approved', 'rejected')),
    approved_by                UUID        REFERENCES users(id) ON DELETE SET NULL,
    approved_at                TIMESTAMPTZ,

    created_by                 UUID        NOT NULL REFERENCES users(id),
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One attendance record per employee per day
    CONSTRAINT uq_hrm_att_employee_date UNIQUE (employee_id, attendance_date)
);

CREATE INDEX idx_hrm_att_org_id      ON hrm_attendance_records (org_id);
CREATE INDEX idx_hrm_att_employee_id ON hrm_attendance_records (employee_id);
CREATE INDEX idx_hrm_att_date        ON hrm_attendance_records (attendance_date);
CREATE INDEX idx_hrm_att_month       ON hrm_attendance_records (org_id, EXTRACT(YEAR FROM attendance_date), EXTRACT(MONTH FROM attendance_date));
CREATE INDEX idx_hrm_att_status      ON hrm_attendance_records (org_id, status) WHERE status = 'pending';

COMMENT ON TABLE  hrm_attendance_records IS 'Daily employee attendance records; one per employee per day';
COMMENT ON COLUMN hrm_attendance_records.shift_id       IS 'FK to A5 hrm_shifts; NULL for unshifted / flexible employees';
COMMENT ON COLUMN hrm_attendance_records.shift_name     IS 'Snapshot at record time; immune to retroactive shift changes';
COMMENT ON COLUMN hrm_attendance_records.regular_hours  IS 'Computed: MIN(duration - break, shift_hours); updated on save';
COMMENT ON COLUMN hrm_attendance_records.overtime_hours IS 'Computed: duration beyond shift_hours if track_overtime=TRUE';
COMMENT ON COLUMN hrm_attendance_records.source         IS 'manual: HR/admin entry; device: biometric/card; api: integration; system: auto-generated';
COMMENT ON COLUMN hrm_attendance_records.status         IS 'approved=normal; pending=awaiting regularization review; rejected=regularization denied';

-- ------------------------------------------------------------
-- hrm_attendance_periods
-- ------------------------------------------------------------
CREATE TABLE hrm_attendance_periods (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id        TEXT        NOT NULL UNIQUE
                                     DEFAULT ('atp_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id           UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    period_year      INTEGER     NOT NULL CHECK (period_year >= 2020),
    period_month     INTEGER     NOT NULL CHECK (period_month BETWEEN 1 AND 12),

    -- Lifecycle: open → finalized (payroll can now run) → locked (payslips paid, immutable)
    status           TEXT        NOT NULL DEFAULT 'open'
                                     CHECK (status IN ('open', 'finalized', 'locked')),

    -- Summary stats computed at finalization (snapshot from attendance_records)
    total_employees  INTEGER     NOT NULL DEFAULT 0,
    total_work_days  INTEGER     NOT NULL DEFAULT 0,  -- org-level working days in month
    total_present    INTEGER     NOT NULL DEFAULT 0,
    total_absent     INTEGER     NOT NULL DEFAULT 0,
    total_holidays   INTEGER     NOT NULL DEFAULT 0,
    total_leaves     INTEGER     NOT NULL DEFAULT 0,
    total_overtime_hours NUMERIC(10,2) NOT NULL DEFAULT 0,

    finalized_at     TIMESTAMPTZ,
    finalized_by     UUID        REFERENCES users(id) ON DELETE SET NULL,
    locked_at        TIMESTAMPTZ,
    locked_by        UUID        REFERENCES users(id) ON DELETE SET NULL,

    created_by       UUID        NOT NULL REFERENCES users(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One period per org per month
    CONSTRAINT uq_hrm_atp_org_month UNIQUE (org_id, period_year, period_month)
);

CREATE INDEX idx_hrm_atp_org_id ON hrm_attendance_periods (org_id);
CREATE INDEX idx_hrm_atp_status ON hrm_attendance_periods (org_id, status);

COMMENT ON TABLE  hrm_attendance_periods IS 'Monthly attendance lock; D2 payslips require status=finalized before computing';
COMMENT ON COLUMN hrm_attendance_periods.status          IS 'open=editable; finalized=payroll can run; locked=payslips paid, no more edits';
COMMENT ON COLUMN hrm_attendance_periods.total_work_days IS 'Org-level working days in the month (excludes weekends/holidays)';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_attendance_periods;
DROP TABLE IF EXISTS hrm_attendance_records;

-- +goose StatementEnd
