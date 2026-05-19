-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00011_create_audit_logs
-- Stores security and business audit trail.
-- Append-only by application convention.
-- ============================================================

CREATE TABLE audit_logs (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT        NOT NULL UNIQUE DEFAULT ('audit_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),

    org_id          UUID        REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         UUID        REFERENCES users(id) ON DELETE SET NULL,

    event_type      TEXT        NOT NULL,
    description     TEXT,

    resource_type   TEXT,
    resource_id     TEXT,

    changes         JSONB,
    metadata        JSONB       NOT NULL DEFAULT '{}'::JSONB,

    ip_address      INET,
    user_agent      TEXT,

    status          TEXT        CHECK (status IN ('success', 'failure', 'warning')),
    error_message   TEXT,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_org_id ON audit_logs (org_id);
CREATE INDEX idx_audit_logs_user_id ON audit_logs (user_id);
CREATE INDEX idx_audit_logs_event_type ON audit_logs (event_type);
CREATE INDEX idx_audit_logs_resource ON audit_logs (resource_type, resource_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs (created_at);

COMMENT ON TABLE audit_logs IS 'Security and business audit events';
COMMENT ON COLUMN audit_logs.event_type IS 'Event key, e.g. auth.sign_in, settings.updated, billing.subscription_changed';
COMMENT ON COLUMN audit_logs.changes IS 'Before/after change details where applicable';
COMMENT ON COLUMN audit_logs.metadata IS 'Additional structured event context';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS audit_logs;

-- +goose StatementEnd
