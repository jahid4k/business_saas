-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00023_hrm_warning_types
--
-- HRM warning type configuration (Group A3):
--   hrm_warning_types            — configurable warning categories
--   hrm_warning_escalation_rules — what happens when threshold is reached
--
-- Design notes:
--   • This migration defines the CONFIG layer only.
--     Warning records (actual instances) live in migration 00032 (Group C1).
--   • severity_level: 1 (mild) → 10 (severe) — used for reporting/sorting.
--   • can_be_issued_by: stored as TEXT[] (e.g. '{admin,manager}').
--   • Escalation action is always 'notify_*' — the system flags, never auto-creates.
--     This was an explicit design decision: HR must review before next action.
--   • valid_duration_days = 0 means the warning never expires.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_warning_types
-- ------------------------------------------------------------
CREATE TABLE hrm_warning_types (
    id                        UUID      PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id                 TEXT      NOT NULL UNIQUE
                                            DEFAULT ('wt_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                    UUID      NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name                      TEXT      NOT NULL,
    description               TEXT,

    -- Severity: 1 (minor counselling) to 10 (final warning before termination)
    severity_level            INTEGER   NOT NULL DEFAULT 5
                                            CHECK (severity_level BETWEEN 1 AND 10),

    -- Roles that may issue this warning type (e.g. '{admin,manager}')
    can_be_issued_by          TEXT[]    NOT NULL DEFAULT '{admin,hr_manager}',

    -- Approval before issuing
    requires_hr_approval      BOOLEAN   NOT NULL DEFAULT FALSE,
    approval_template_id      UUID      REFERENCES hrm_approval_templates(id) ON DELETE SET NULL,

    -- Employee response window
    employee_can_respond      BOOLEAN   NOT NULL DEFAULT TRUE,
    response_window_days      INTEGER   NOT NULL DEFAULT 5 CHECK (response_window_days >= 0),

    -- Auto-generate a warning letter from a document template
    auto_generate_document    BOOLEAN   NOT NULL DEFAULT FALSE,
    document_template_id      UUID,     -- FK added after hrm_document_templates exists (migration 00024)

    -- How long this warning counts toward escalation rules
    -- 0 = permanent (never expires from the employee record)
    valid_duration_days       INTEGER   NOT NULL DEFAULT 0 CHECK (valid_duration_days >= 0),

    is_active                 BOOLEAN   NOT NULL DEFAULT TRUE,

    created_by                UUID      NOT NULL REFERENCES users(id),
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_wt_org_id ON hrm_warning_types (org_id);
CREATE UNIQUE INDEX idx_hrm_wt_org_name ON hrm_warning_types (org_id, LOWER(name)) WHERE is_active = TRUE;

COMMENT ON TABLE  hrm_warning_types IS 'Configurable warning category definitions per organization';
COMMENT ON COLUMN hrm_warning_types.severity_level       IS '1 = minor counselling, 10 = final warning before termination';
COMMENT ON COLUMN hrm_warning_types.can_be_issued_by     IS 'Array of role names allowed to issue this warning type';
COMMENT ON COLUMN hrm_warning_types.valid_duration_days  IS '0 = permanent; > 0 = expires after N days from issue date';

-- ------------------------------------------------------------
-- hrm_warning_escalation_rules
-- ------------------------------------------------------------
CREATE TABLE hrm_warning_escalation_rules (
    id                       UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id                TEXT    NOT NULL UNIQUE
                                         DEFAULT ('wer_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                   UUID    NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Trigger: when this many warnings of trigger_warning_type_id are active...
    trigger_warning_type_id  UUID    NOT NULL REFERENCES hrm_warning_types(id) ON DELETE CASCADE,
    trigger_count            INTEGER NOT NULL DEFAULT 3 CHECK (trigger_count >= 1),
    within_days              INTEGER NOT NULL DEFAULT 0 CHECK (within_days >= 0),
                                     -- 0 = count all time, > 0 = only within last N days

    -- Action: what the system does (always notify — never auto-create)
    action                   TEXT    NOT NULL DEFAULT 'notify_hr'
                                         CHECK (action IN
                                             ('notify_hr', 'notify_management', 'flag_termination_review')),
    notification_roles       TEXT[]  NOT NULL DEFAULT '{admin}',

    is_active                BOOLEAN NOT NULL DEFAULT TRUE,

    created_by               UUID    NOT NULL REFERENCES users(id),
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_wer_org_id  ON hrm_warning_escalation_rules (org_id);
CREATE INDEX idx_hrm_wer_type_id ON hrm_warning_escalation_rules (trigger_warning_type_id);

COMMENT ON TABLE  hrm_warning_escalation_rules IS 'Threshold rules that trigger HR alerts when warning count is reached';
COMMENT ON COLUMN hrm_warning_escalation_rules.within_days IS '0 = count all-time; > 0 = sliding window in days';
COMMENT ON COLUMN hrm_warning_escalation_rules.action      IS 'System flags HR — never auto-creates next warning (by design)';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_warning_escalation_rules;
DROP TABLE IF EXISTS hrm_warning_types;

-- +goose StatementEnd
