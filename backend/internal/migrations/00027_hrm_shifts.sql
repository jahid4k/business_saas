-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00025_hrm_shifts
--
-- HRM work schedule configuration (Group A5):
--   hrm_shifts                     — shift definitions (fixed or flexible)
--   hrm_work_schedule_assignments  — assigns shifts to org / dept / employee
--
-- Design notes:
--   • shift_type = 'fixed': start_time + end_time are required.
--   • shift_type = 'flexible': core_start_time + core_end_time define the
--     window employees must be online; total hours target from weekly_hours_target.
--   • working_days is stored as TEXT[] e.g. '{mon,tue,wed,thu,fri}'.
--   • Lookup priority on assignment: employee > department > organization.
--     The attendance engine resolves this at computation time.
--   • effective_date + end_date allow scheduling shift changes in advance.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_shifts
-- ------------------------------------------------------------
CREATE TABLE hrm_shifts (
    id                   UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id            TEXT    NOT NULL UNIQUE
                                     DEFAULT ('sh_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id               UUID    NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name                 TEXT    NOT NULL,
    description          TEXT,
    shift_type           TEXT    NOT NULL DEFAULT 'fixed'
                                     CHECK (shift_type IN ('fixed', 'flexible')),

    -- Fixed shift fields (required when shift_type = 'fixed')
    start_time           TIME,   -- e.g. '09:00:00'
    end_time             TIME,   -- e.g. '18:00:00'

    -- Flexible shift fields (required when shift_type = 'flexible')
    core_start_time      TIME,   -- must be online FROM
    core_end_time        TIME,   -- must be online UNTIL
    weekly_hours_target  NUMERIC(5,2),  -- total hours per week expected

    break_minutes        INTEGER NOT NULL DEFAULT 60 CHECK (break_minutes >= 0),

    -- Which days of the week are working days
    -- Values must be subset of: {mon,tue,wed,thu,fri,sat,sun}
    working_days         TEXT[]  NOT NULL DEFAULT '{mon,tue,wed,thu,fri}',

    -- Overtime and lateness tracking
    track_overtime       BOOLEAN NOT NULL DEFAULT FALSE,
    overtime_threshold_hours NUMERIC(5,2),  -- hours/day beyond which it's OT

    track_breaks         BOOLEAN NOT NULL DEFAULT FALSE,  -- per-punch break vs fixed break_minutes

    is_default           BOOLEAN NOT NULL DEFAULT FALSE,
    is_active            BOOLEAN NOT NULL DEFAULT TRUE,

    created_by           UUID    NOT NULL REFERENCES users(id),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_sh_fixed
        CHECK (shift_type != 'fixed' OR (start_time IS NOT NULL AND end_time IS NOT NULL)),
    CONSTRAINT chk_hrm_sh_flexible
        CHECK (shift_type != 'flexible' OR (weekly_hours_target IS NOT NULL))
);

CREATE INDEX idx_hrm_sh_org_id ON hrm_shifts (org_id);
CREATE UNIQUE INDEX idx_hrm_sh_org_name ON hrm_shifts (org_id, LOWER(name)) WHERE is_active = TRUE;
CREATE UNIQUE INDEX idx_hrm_sh_org_default ON hrm_shifts (org_id) WHERE is_default = TRUE AND is_active = TRUE;

COMMENT ON TABLE  hrm_shifts IS 'Shift definitions: fixed (set hours) or flexible (core window + weekly target)';
COMMENT ON COLUMN hrm_shifts.working_days       IS 'Subset of {mon,tue,wed,thu,fri,sat,sun}; drives absence detection';
COMMENT ON COLUMN hrm_shifts.track_breaks       IS 'If true, break duration computed from punches; else uses break_minutes';

-- ------------------------------------------------------------
-- hrm_work_schedule_assignments
-- ------------------------------------------------------------
CREATE TABLE hrm_work_schedule_assignments (
    id              UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT    NOT NULL UNIQUE
                                DEFAULT ('wsa_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id          UUID    NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    shift_id        UUID    NOT NULL REFERENCES hrm_shifts(id) ON DELETE CASCADE,

    -- Polymorphic: who this assignment applies to
    assignee_type   TEXT    NOT NULL CHECK (assignee_type IN ('organization', 'department', 'employee')),
    assignee_id     UUID    NOT NULL,  -- org_id | dept_id | employee_id depending on type

    effective_date  DATE    NOT NULL DEFAULT CURRENT_DATE,
    end_date        DATE,   -- NULL = indefinite

    created_by      UUID    NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_wsa_dates CHECK (end_date IS NULL OR end_date >= effective_date)
);

CREATE INDEX idx_hrm_wsa_org_id      ON hrm_work_schedule_assignments (org_id);
CREATE INDEX idx_hrm_wsa_shift_id    ON hrm_work_schedule_assignments (shift_id);
CREATE INDEX idx_hrm_wsa_assignee    ON hrm_work_schedule_assignments (assignee_type, assignee_id);
CREATE INDEX idx_hrm_wsa_effective   ON hrm_work_schedule_assignments (assignee_type, assignee_id, effective_date DESC);

COMMENT ON TABLE  hrm_work_schedule_assignments IS 'Assigns shifts to org/dept/employee with date range support';
COMMENT ON COLUMN hrm_work_schedule_assignments.assignee_type IS 'organization > department > employee priority for lookup';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_work_schedule_assignments;
DROP TABLE IF EXISTS hrm_shifts;

-- +goose StatementEnd
