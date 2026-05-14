-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00007_create_audit_logs
-- Creates the append-only audit log table.
--
-- Rules:
--   - Rows are NEVER updated or deleted
--   - No cascading foreign keys — the log must survive user/
--     business deletion (user_id and business_id are nullable)
--   - Written asynchronously — failures never block requests
--   - metadata is JSONB — each event type stores its own shape
--
-- Events logged (from audit/service.go):
--   auth.signup, auth.login, auth.login_failed,
--   auth.logout, auth.logout_all,
--   auth.password_reset_request, auth.password_reset_confirm,
--   business.created, authz.role_assigned,
--   task.created, task.deleted
-- ============================================================

CREATE TABLE audit_logs (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID,                  -- NULL for anonymous/system events
    business_id UUID,                  -- NULL for pre-business events (signup)
    event_type  TEXT        NOT NULL,  -- e.g. 'auth.login', 'task.deleted'
    metadata    JSONB       NOT NULL DEFAULT '{}',
    ip_address  TEXT        NOT NULL DEFAULT '',
    user_agent  TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- NOTE: No FK constraints on user_id or business_id.
-- The audit log must survive deletes of the entities it references.
-- We store the IDs for correlation, not for referential integrity.

-- Time-range queries: "show all events in the last 24 hours"
CREATE INDEX idx_audit_logs_created_at
    ON audit_logs (created_at DESC);

-- Per-user audit trail: "show all events for user X"
CREATE INDEX idx_audit_logs_user_id_created_at
    ON audit_logs (user_id, created_at DESC)
    WHERE user_id IS NOT NULL;

-- Per-business audit trail: "show all events in business X"
CREATE INDEX idx_audit_logs_business_id_created_at
    ON audit_logs (business_id, created_at DESC)
    WHERE business_id IS NOT NULL;

-- Filter by event type: "show all failed logins"
CREATE INDEX idx_audit_logs_event_type_created_at
    ON audit_logs (event_type, created_at DESC);

-- JSONB index — enables fast queries on metadata fields
-- e.g. WHERE metadata->>'target_user_id' = '...'
CREATE INDEX idx_audit_logs_metadata
    ON audit_logs USING GIN (metadata);

COMMENT ON TABLE  audit_logs            IS 'Append-only security audit log — rows are never updated or deleted';
COMMENT ON COLUMN audit_logs.user_id    IS 'Actor — NULL for anonymous events. No FK — survives user deletion';
COMMENT ON COLUMN audit_logs.business_id IS 'Context — NULL for global events. No FK — survives business deletion';
COMMENT ON COLUMN audit_logs.event_type IS 'Machine-readable event identifier e.g. auth.login, task.deleted';
COMMENT ON COLUMN audit_logs.metadata   IS 'JSONB payload — shape varies per event_type';

-- ----------------------------------------------------------
-- Prevent accidental updates/deletes via a trigger
-- The application should never do this, but this makes it
-- impossible at the database level.
-- ----------------------------------------------------------

CREATE OR REPLACE FUNCTION audit_logs_immutable()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs is append-only — updates and deletes are not permitted';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_logs_no_update
    BEFORE UPDATE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION audit_logs_immutable();

CREATE TRIGGER audit_logs_no_delete
    BEFORE DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION audit_logs_immutable();

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS audit_logs_no_delete ON audit_logs;
DROP TRIGGER IF EXISTS audit_logs_no_update ON audit_logs;
DROP FUNCTION IF EXISTS audit_logs_immutable();
DROP TABLE IF EXISTS audit_logs;

-- +goose StatementEnd