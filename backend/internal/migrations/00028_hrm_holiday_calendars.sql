-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00026_hrm_holiday_calendars
--
-- HRM holiday calendar management (Group A6):
--   hrm_holiday_calendars   — named collections of holidays (annual)
--   hrm_holidays            — individual holiday entries within a calendar
--   hrm_calendar_assignments — assigns calendars to org / dept / employee
--
-- Design notes:
--   • Lookup priority: employee → department → organization.
--   • holiday_type = 'optional': employees may choose to work; is_paid optional.
--   • repeat_yearly = TRUE: in the next year, generate the same date automatically
--     (e.g. December 25 every year). The engine creates the new year's holiday
--     at the start of each calendar year via a scheduled job.
--   • Leave calculation uses the employee's active calendar to determine
--     working days in a period — holidays are excluded automatically.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_holiday_calendars
-- ------------------------------------------------------------
CREATE TABLE hrm_holiday_calendars (
    id           UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id    TEXT    NOT NULL UNIQUE
                             DEFAULT ('hc_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id       UUID    NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name         TEXT    NOT NULL,
    description  TEXT,
    country_code TEXT,   -- ISO 3166-1 alpha-2 (optional, for display)
    year         INTEGER NOT NULL CHECK (year BETWEEN 2000 AND 2100),

    is_active    BOOLEAN NOT NULL DEFAULT TRUE,

    created_by   UUID    NOT NULL REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_hc_org_id ON hrm_holiday_calendars (org_id);
CREATE UNIQUE INDEX idx_hrm_hc_org_year_name
    ON hrm_holiday_calendars (org_id, year, LOWER(name));

COMMENT ON TABLE hrm_holiday_calendars IS 'Named annual holiday collections per organization';

-- ------------------------------------------------------------
-- hrm_holidays
-- ------------------------------------------------------------
CREATE TABLE hrm_holidays (
    id              UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT    NOT NULL UNIQUE
                                DEFAULT ('hd_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    calendar_id     UUID    NOT NULL REFERENCES hrm_holiday_calendars(id) ON DELETE CASCADE,

    name            TEXT    NOT NULL,
    date            DATE    NOT NULL,

    holiday_type    TEXT    NOT NULL DEFAULT 'public'
                                CHECK (holiday_type IN ('public', 'company', 'optional')),
    is_paid         BOOLEAN NOT NULL DEFAULT TRUE,

    -- If true, a job creates the equivalent holiday in next year's calendar
    repeat_yearly   BOOLEAN NOT NULL DEFAULT FALSE,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_hrm_hd_calendar_date UNIQUE (calendar_id, date)
);

CREATE INDEX idx_hrm_hd_calendar_id ON hrm_holidays (calendar_id);
CREATE INDEX idx_hrm_hd_date        ON hrm_holidays (date);

COMMENT ON TABLE  hrm_holidays IS 'Individual holiday entries within a calendar';
COMMENT ON COLUMN hrm_holidays.repeat_yearly IS 'Triggers automatic carry-forward to next year on the same date';

-- ------------------------------------------------------------
-- hrm_calendar_assignments
-- ------------------------------------------------------------
CREATE TABLE hrm_calendar_assignments (
    id              UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT    NOT NULL UNIQUE
                                DEFAULT ('ca_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id          UUID    NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    calendar_id     UUID    NOT NULL REFERENCES hrm_holiday_calendars(id) ON DELETE CASCADE,

    -- Polymorphic assignee
    assignee_type   TEXT    NOT NULL CHECK (assignee_type IN ('organization', 'department', 'employee')),
    assignee_id     UUID    NOT NULL,

    effective_date  DATE    NOT NULL DEFAULT CURRENT_DATE,

    created_by      UUID    NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Only one active calendar per assignee at a time
    CONSTRAINT uq_hrm_ca_assignee UNIQUE (assignee_type, assignee_id)
);

CREATE INDEX idx_hrm_ca_org_id    ON hrm_calendar_assignments (org_id);
CREATE INDEX idx_hrm_ca_calendar  ON hrm_calendar_assignments (calendar_id);
CREATE INDEX idx_hrm_ca_assignee  ON hrm_calendar_assignments (assignee_type, assignee_id);

COMMENT ON TABLE  hrm_calendar_assignments IS 'Assigns holiday calendars to org/dept/employee';
COMMENT ON COLUMN hrm_calendar_assignments.assignee_type IS 'Lookup priority: employee → department → organization';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_calendar_assignments;
DROP TABLE IF EXISTS hrm_holidays;
DROP TABLE IF EXISTS hrm_holiday_calendars;

-- +goose StatementEnd
