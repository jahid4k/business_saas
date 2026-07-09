-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00035_hrm_complaints
--
-- HRM employee complaints and grievance management — Group C2.
--
-- Status machine:
--   submitted → under_review → investigating → resolved
--                                                 ↓
--                                              dismissed
--   (employee may withdraw at any pre-resolution stage)
--
-- Design notes:
--   • against_employee_id is nullable — complaints may be against
--     the organization, a policy, or conditions rather than a person.
--   • is_anonymous = TRUE hides the complainant's identity from
--     non-HR users (enforced at the handler layer via permission check).
--   • document_id (A4) links to a resolution/acknowledgement letter.
--   • investigator_id is the HR or neutral party assigned to investigate.
-- ============================================================

CREATE TABLE hrm_complaints (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id             TEXT        NOT NULL UNIQUE
                                          DEFAULT ('cpl_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Complainant
    employee_id           UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE RESTRICT,
    is_anonymous          BOOLEAN     NOT NULL DEFAULT FALSE,

    -- Complaint subject
    complaint_type        TEXT        NOT NULL DEFAULT 'general'
                                          CHECK (complaint_type IN (
                                              'harassment', 'discrimination', 'workplace_safety',
                                              'policy_violation', 'manager_conduct',
                                              'wage_dispute', 'retaliation', 'general'
                                          )),
    title                 TEXT        NOT NULL,
    description           TEXT        NOT NULL,
    incident_date         DATE,

    -- Who the complaint is against (nullable — may be against policy/environment)
    against_employee_id   UUID        REFERENCES hrm_employees(id) ON DELETE SET NULL,
    against_details       TEXT,       -- free-text if against person is not an employee

    -- Investigation
    investigator_id       UUID        REFERENCES users(id) ON DELETE SET NULL,
    investigation_notes   TEXT,
    investigation_started_at TIMESTAMPTZ,

    -- Resolution
    resolution            TEXT,
    resolution_action     TEXT        CHECK (resolution_action IN (
                              'warning_issued', 'termination', 'policy_updated',
                              'mediation', 'training', 'no_action', 'other', NULL
                          )),
    resolved_at           TIMESTAMPTZ,
    resolved_by           UUID        REFERENCES users(id) ON DELETE SET NULL,

    -- A4: resolution or outcome letter
    document_id           UUID        REFERENCES hrm_employee_documents(id) ON DELETE SET NULL,

    status                TEXT        NOT NULL DEFAULT 'submitted'
                                          CHECK (status IN (
                                              'submitted', 'under_review', 'investigating',
                                              'resolved', 'dismissed', 'withdrawn'
                                          )),

    created_by            UUID        NOT NULL REFERENCES users(id),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_cpl_org_id      ON hrm_complaints (org_id);
CREATE INDEX idx_hrm_cpl_employee_id ON hrm_complaints (employee_id);
CREATE INDEX idx_hrm_cpl_against     ON hrm_complaints (against_employee_id) WHERE against_employee_id IS NOT NULL;
CREATE INDEX idx_hrm_cpl_status      ON hrm_complaints (org_id, status);
CREATE INDEX idx_hrm_cpl_investigator ON hrm_complaints (investigator_id) WHERE investigator_id IS NOT NULL;

COMMENT ON TABLE  hrm_complaints IS 'Employee grievance and complaint records';
COMMENT ON COLUMN hrm_complaints.is_anonymous         IS 'When true, complainant identity hidden from non-HR users at handler level';
COMMENT ON COLUMN hrm_complaints.against_employee_id  IS 'NULL if complaint is against policy, conditions, or non-employee';
COMMENT ON COLUMN hrm_complaints.resolution_action    IS 'What concrete action was taken to resolve the complaint';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS hrm_complaints;
-- +goose StatementEnd
