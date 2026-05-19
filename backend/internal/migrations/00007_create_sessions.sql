-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00007_create_sessions
-- Stores active/revocable user sessions and device information.
-- If Auth.js JWT-only sessions are used, this table is still useful
-- for security pages, device management, and refresh-token tracking.
-- ============================================================

CREATE TABLE sessions (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id           TEXT        NOT NULL UNIQUE DEFAULT ('sess_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),

    user_id             UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id              UUID        REFERENCES organizations(id) ON DELETE CASCADE,

    token_hash          TEXT        NOT NULL UNIQUE,

    device_name         TEXT,
    device_type         TEXT,
    browser             TEXT,
    os                  TEXT,

    user_agent          TEXT,
    ip_address          INET,

    country             TEXT,
    city                TEXT,
    region              TEXT,

    last_activity_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at          TIMESTAMPTZ NOT NULL,
    revoked_at          TIMESTAMPTZ
);

CREATE INDEX idx_sessions_user_id ON sessions (user_id);
CREATE INDEX idx_sessions_org_id ON sessions (org_id);
CREATE INDEX idx_sessions_token_hash ON sessions (token_hash);
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);
CREATE INDEX idx_sessions_revoked_at ON sessions (revoked_at);

COMMENT ON TABLE sessions IS 'Active and historical user sessions/devices';
COMMENT ON COLUMN sessions.token_hash IS 'Hash of session/refresh token; never store raw token';
COMMENT ON COLUMN sessions.revoked_at IS 'Non-null when user/admin revoked this session';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS sessions;

-- +goose StatementEnd
