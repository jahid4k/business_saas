-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00036_hrm_acknowledgements
--
-- Generic employee acknowledgement system — Group C4.
-- Cross-cutting: used by C1 (warnings), C3 (documents),
-- E2 (announcements), and E3 (HR calendar events).
--
-- Design notes:
--   • acknowledgeable_type + acknowledgeable_id = polymorphic FK.
--     The type discriminates which table to look up.
--   • signature_required: when TRUE, signed_at + signature_data
--     must be populated before the acknowledgement is considered complete.
--   • One acknowledgement record per (employee, entity) pair.
--     Re-acknowledging after expiry creates a new record
--     (the old one is NOT deleted — audit trail).
--   • For simple acknowledgements (reading a policy), signature_required
--     is FALSE and the employee just clicks "I acknowledge".
--   • For formal document signing, signature_required = TRUE and
--     signature_data stores a base64 image or typed name (implementation choice).
--   • expires_at: when set, the acknowledgement must be re-done after this date.
--     Useful for annual policy re-acknowledgements.
--
-- Cross-module usage:
--   C1 Warnings:       acknowledgeable_type = 'warning'
--   C3 Emp Docs:       acknowledgeable_type = 'document'
--   E2 Announcements:  acknowledgeable_type = 'announcement'
--   E3 HR Calendar:    acknowledgeable_type = 'calendar_event' (RSVP)
-- ============================================================

CREATE TABLE hrm_acknowledgements (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id             TEXT        NOT NULL UNIQUE
                                          DEFAULT ('ack_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Who is acknowledging
    employee_id           UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE RESTRICT,

    -- What they are acknowledging (polymorphic)
    acknowledgeable_type  TEXT        NOT NULL
                                          CHECK (acknowledgeable_type IN (
                                              'warning', 'document', 'announcement',
                                              'calendar_event', 'policy'
                                          )),
    acknowledgeable_id    UUID        NOT NULL,

    -- Content snapshot (denormalized for audit — the entity may change)
    entity_title          TEXT        NOT NULL,   -- e.g. "Written Warning — Attendance", "Annual Leave Policy 2026"

    -- Acknowledgement details
    notes                 TEXT,
    signature_required    BOOLEAN     NOT NULL DEFAULT FALSE,
    signed_at             TIMESTAMPTZ,
    signature_data        TEXT,       -- base64 image or typed name; NULL for non-signature acks

    -- Status
    status                TEXT        NOT NULL DEFAULT 'pending'
                                          CHECK (status IN ('pending', 'acknowledged', 'declined', 'expired')),
    acknowledged_at       TIMESTAMPTZ,
    declined_at           TIMESTAMPTZ,
    decline_reason        TEXT,

    -- Expiry (for annual re-acknowledgement)
    expires_at            DATE,

    -- Who sent the acknowledgement request
    requested_by          UUID        NOT NULL REFERENCES users(id),
    requested_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reminder_sent_at      TIMESTAMPTZ,

    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One active acknowledgement per (employee, entity) — allow multiple over time for expiring ones
    CONSTRAINT uq_hrm_ack_active
        UNIQUE (employee_id, acknowledgeable_type, acknowledgeable_id, status)
        DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX idx_hrm_ack_org_id     ON hrm_acknowledgements (org_id);
CREATE INDEX idx_hrm_ack_employee   ON hrm_acknowledgements (employee_id);
CREATE INDEX idx_hrm_ack_entity     ON hrm_acknowledgements (acknowledgeable_type, acknowledgeable_id);
CREATE INDEX idx_hrm_ack_status     ON hrm_acknowledgements (org_id, status);
CREATE INDEX idx_hrm_ack_pending    ON hrm_acknowledgements (employee_id, status) WHERE status = 'pending';
CREATE INDEX idx_hrm_ack_expires    ON hrm_acknowledgements (expires_at) WHERE expires_at IS NOT NULL AND status = 'acknowledged';

COMMENT ON TABLE  hrm_acknowledgements IS 'Cross-cutting employee acknowledgement system: warnings, docs, announcements, events';
COMMENT ON COLUMN hrm_acknowledgements.acknowledgeable_type IS 'Discriminator for polymorphic FK: warning|document|announcement|calendar_event|policy';
COMMENT ON COLUMN hrm_acknowledgements.entity_title         IS 'Snapshot of entity title at time of request — survives entity edits';
COMMENT ON COLUMN hrm_acknowledgements.signature_required   IS 'When true, signed_at + signature_data required for completion';
COMMENT ON COLUMN hrm_acknowledgements.expires_at           IS 'When set, employee must re-acknowledge after this date';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS hrm_acknowledgements;
-- +goose StatementEnd
