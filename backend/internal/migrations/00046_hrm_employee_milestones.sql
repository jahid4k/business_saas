-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00044_hrm_employee_milestones
--
-- HRM employee milestones — Group E4.
-- Depends on: A7 (contract dates), E1 (auto-award), E2 (auto-announce), E3 (auto-event)
--
-- Milestone types:
--   work_anniversary   → 1yr, 2yr, 5yr, 10yr, etc. (from hire_date)
--   birthday           → annual birthday (from dob in hrm_employees)
--   probation_complete → from hrm_employee_contracts.probation_end_date (A7)
--   promotion          → triggered by B1 promotions when applied
--   contract_renewal   → from hrm_employee_contracts.end_date (A7)
--   retirement         → configured or from B3/B4 status changes
--   custom             → HR-defined
--
-- Auto-generation:
--   The service.GenerateUpcoming() method creates milestones for a given
--   org + month by scanning active employees' hire dates and DOBs.
--   HR may also create milestones manually.
--
-- Cascade actions (all optional, controlled by flags):
--   auto_award_id          → E1 award created when milestone is triggered
--   auto_announcement_id   → E2 announcement created
--   auto_calendar_event_id → E3 calendar event created
--
-- Uniqueness: one milestone per (employee, type, milestone_date).
-- ============================================================

CREATE TABLE hrm_employee_milestones (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id                TEXT        NOT NULL UNIQUE
                                             DEFAULT ('mil_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                   UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id              UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE RESTRICT,

    milestone_type           TEXT        NOT NULL
                                             CHECK (milestone_type IN (
                                                 'work_anniversary', 'birthday', 'probation_complete',
                                                 'promotion', 'contract_renewal', 'retirement', 'custom'
                                             )),

    title                    TEXT        NOT NULL,
    description              TEXT,

    milestone_date           DATE        NOT NULL,
    years_count              INTEGER,    -- e.g. 5 for a 5-year work anniversary; NULL for non-anniversary types

    -- How it was created
    is_auto_generated        BOOLEAN     NOT NULL DEFAULT FALSE,

    -- Cascade links (populated after the linked record is created)
    auto_award_id            UUID        REFERENCES hrm_awards(id) ON DELETE SET NULL,
    auto_announcement_id     UUID        REFERENCES hrm_announcements(id) ON DELETE SET NULL,
    auto_calendar_event_id   UUID        REFERENCES hrm_calendar_events(id) ON DELETE SET NULL,

    -- Acknowledgement tracking (did HR/employee acknowledge/celebrate this?)
    is_acknowledged          BOOLEAN     NOT NULL DEFAULT FALSE,
    acknowledged_at          TIMESTAMPTZ,

    created_by               UUID        NOT NULL REFERENCES users(id),
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One milestone per employee per type per date
    CONSTRAINT uq_hrm_mil_employee_type_date
        UNIQUE (employee_id, milestone_type, milestone_date)
);

CREATE INDEX idx_hrm_mil_org_id      ON hrm_employee_milestones (org_id);
CREATE INDEX idx_hrm_mil_employee_id ON hrm_employee_milestones (employee_id);
CREATE INDEX idx_hrm_mil_type        ON hrm_employee_milestones (org_id, milestone_type);
CREATE INDEX idx_hrm_mil_date        ON hrm_employee_milestones (org_id, milestone_date);
CREATE INDEX idx_hrm_mil_upcoming    ON hrm_employee_milestones (org_id, milestone_date)
    WHERE is_acknowledged = FALSE;

COMMENT ON TABLE  hrm_employee_milestones IS 'Employee lifecycle milestones: anniversaries, birthdays, promotions, etc.';
COMMENT ON COLUMN hrm_employee_milestones.years_count         IS 'Year number for anniversary types (e.g. 5 for 5-year anniversary)';
COMMENT ON COLUMN hrm_employee_milestones.is_auto_generated   IS 'TRUE when created by GenerateUpcoming() from hire_date/DOB scan';
COMMENT ON COLUMN hrm_employee_milestones.auto_award_id       IS 'E1 award created as part of milestone cascade; NULL if not configured';
COMMENT ON COLUMN hrm_employee_milestones.auto_announcement_id IS 'E2 announcement created as part of milestone cascade';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS hrm_employee_milestones;
-- +goose StatementEnd
