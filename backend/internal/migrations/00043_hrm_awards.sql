-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00041_hrm_awards
--
-- HRM employee recognition and awards — Group E1.
-- Depends on: A2 (optional approval), A4 (certificate document)
--
-- Status machine:
--   draft → pending_approval → approved → issued → cancelled
--   (approval skipped when approval_instance_id is NULL)
--
-- Design notes:
--   • When issued, the service optionally creates a linked E2
--     announcement (announcement_id FK added in 00042 after that
--     table exists — see ALTER TABLE in 00042).
--   • monetary_value is informational — disbursed via payroll/accounting.
--   • points are for gamification/recognition programs; tracked but
--     no redemption engine in this group.
-- ============================================================

CREATE TABLE hrm_awards (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id                TEXT        NOT NULL UNIQUE
                                             DEFAULT ('awd_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                   UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id              UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE RESTRICT,

    award_type               TEXT        NOT NULL DEFAULT 'spot_recognition'
                                             CHECK (award_type IN (
                                                 'spot_recognition', 'performance', 'tenure',
                                                 'team', 'innovation', 'customer_service', 'custom'
                                             )),

    title                    TEXT        NOT NULL,
    description              TEXT        NOT NULL,

    -- Optional reward values
    points                   INTEGER     NOT NULL DEFAULT 0 CHECK (points >= 0),
    monetary_value           NUMERIC(12,2),
    currency                 TEXT        NOT NULL DEFAULT 'BDT',

    award_date               DATE        NOT NULL DEFAULT CURRENT_DATE,

    -- Who issued
    issued_by                UUID        NOT NULL REFERENCES users(id),

    -- A2: optional approval chain
    approval_instance_id     UUID        REFERENCES hrm_approval_instances(id) ON DELETE SET NULL,

    -- A4: auto-generated certificate (NULL if no template configured)
    certificate_document_id  UUID        REFERENCES hrm_employee_documents(id) ON DELETE SET NULL,

    -- E2: linked announcement (FK added in migration 00042)
    announcement_id          UUID,

    status                   TEXT        NOT NULL DEFAULT 'draft'
                                             CHECK (status IN (
                                                 'draft', 'pending_approval', 'approved',
                                                 'issued', 'cancelled'
                                             )),

    issued_at                TIMESTAMPTZ,

    created_by               UUID        NOT NULL REFERENCES users(id),
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_awd_org_id      ON hrm_awards (org_id);
CREATE INDEX idx_hrm_awd_employee_id ON hrm_awards (employee_id);
CREATE INDEX idx_hrm_awd_status      ON hrm_awards (org_id, status);
CREATE INDEX idx_hrm_awd_type        ON hrm_awards (org_id, award_type);

COMMENT ON TABLE  hrm_awards IS 'Employee recognition records: spot awards, tenure, performance, etc.';
COMMENT ON COLUMN hrm_awards.points           IS 'Recognition points for gamification programs; informational only in this version';
COMMENT ON COLUMN hrm_awards.monetary_value   IS 'Cash/gift value; disbursed via payroll — stored here for HR record only';
COMMENT ON COLUMN hrm_awards.announcement_id  IS 'FK to hrm_announcements added in migration 00042 post-create';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_awards;

-- +goose StatementEnd
