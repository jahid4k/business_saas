-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00034_hrm_employee_warnings
--
-- HRM employee warning records — Group C1.
-- This is the RUNTIME layer. The config layer (warning types,
-- escalation rules) lives in migration 00023 (Group A3).
--
-- Status machine:
--   draft → issued → acknowledged        (employee confirms receipt)
--               ↓       ↓
--             cancelled  appealed → closed (HR reviews appeal)
--
-- Design notes:
--   • warning_type_id references A3 hrm_warning_types; fields like
--     can_employee_respond and response_window_days are SNAPSHOTTED
--     at issuance time so changes to the type config don't
--     retroactively change active warnings.
--   • expires_at is computed at issuance: issued_date + valid_duration_days.
--     NULL = permanent (valid_duration_days was 0).
--   • Escalation check (against A3 rules) is performed by the service
--     at issuance time; the service logs a notification event when
--     a threshold is reached (never auto-creates a new warning).
--   • document_id (A4) is set when a letter is auto-generated from
--     the warning type's document_template_id.
-- ============================================================

CREATE TABLE hrm_employee_warnings (
    id                        UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id                 TEXT        NOT NULL UNIQUE
                                              DEFAULT ('ew_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                    UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id               UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE RESTRICT,

    -- Config snapshot at issuance
    warning_type_id           UUID        NOT NULL REFERENCES hrm_warning_types(id) ON DELETE RESTRICT,
    warning_type_name         TEXT        NOT NULL,  -- snapshot so type rename doesn't change record
    severity_level            INTEGER     NOT NULL DEFAULT 5 CHECK (severity_level BETWEEN 1 AND 10),

    -- Incident details
    title                     TEXT        NOT NULL,
    description               TEXT        NOT NULL,
    incident_date             DATE        NOT NULL,

    -- Issuer
    issued_by                 UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    witness_ids               UUID[]      NOT NULL DEFAULT '{}',

    -- A2: approval before issuing (if warning_type.requires_hr_approval = true)
    approval_instance_id      UUID        REFERENCES hrm_approval_instances(id) ON DELETE SET NULL,

    -- A4: auto-generated warning letter
    document_id               UUID        REFERENCES hrm_employee_documents(id) ON DELETE SET NULL,

    -- Employee response (fields snapshotted from warning_type at creation)
    can_employee_respond      BOOLEAN     NOT NULL DEFAULT TRUE,
    response_window_days      INTEGER     NOT NULL DEFAULT 5 CHECK (response_window_days >= 0),
    response_deadline         DATE,                -- computed: issued_date + response_window_days
    employee_response         TEXT,
    employee_responded_at     TIMESTAMPTZ,

    -- Appeal
    appeal_reason             TEXT,
    appeal_submitted_at       TIMESTAMPTZ,
    appeal_resolution         TEXT,
    appeal_resolved_at        TIMESTAMPTZ,

    -- Expiry (computed: issued_date + valid_duration_days; NULL = permanent)
    expires_at                DATE,
    is_active                 BOOLEAN     NOT NULL DEFAULT TRUE,

    issued_at                 TIMESTAMPTZ,        -- when formally issued (not just created as draft)

    status                    TEXT        NOT NULL DEFAULT 'draft'
                                              CHECK (status IN (
                                                  'draft', 'pending_approval', 'issued',
                                                  'acknowledged', 'appealed', 'closed', 'cancelled'
                                              )),

    created_by                UUID        NOT NULL REFERENCES users(id),
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_ew_org_id       ON hrm_employee_warnings (org_id);
CREATE INDEX idx_hrm_ew_employee_id  ON hrm_employee_warnings (employee_id);
CREATE INDEX idx_hrm_ew_type_id      ON hrm_employee_warnings (warning_type_id);
CREATE INDEX idx_hrm_ew_status       ON hrm_employee_warnings (org_id, status);
CREATE INDEX idx_hrm_ew_active       ON hrm_employee_warnings (employee_id, is_active) WHERE is_active = TRUE;
CREATE INDEX idx_hrm_ew_expires      ON hrm_employee_warnings (expires_at) WHERE expires_at IS NOT NULL AND is_active = TRUE;

COMMENT ON TABLE  hrm_employee_warnings IS 'Issued warning records; config in hrm_warning_types (A3)';
COMMENT ON COLUMN hrm_employee_warnings.warning_type_name IS 'Snapshot at creation — immune to type rename';
COMMENT ON COLUMN hrm_employee_warnings.expires_at        IS 'issued_date + valid_duration_days; NULL = permanent';
COMMENT ON COLUMN hrm_employee_warnings.is_active         IS 'Set FALSE when warning expires; drives escalation count';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS hrm_employee_warnings;
-- +goose StatementEnd
