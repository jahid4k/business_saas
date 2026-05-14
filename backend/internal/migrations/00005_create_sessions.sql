-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00005_create_sessions
-- Persists refresh token sessions and password reset tokens.
--
-- Security model:
--   Raw tokens are NEVER stored.
--   Only SHA-256 hashes of tokens are stored.
--   On verification: hash the incoming token, look it up by hash.
--   A database breach does NOT expose valid tokens.
--
-- Sessions (refresh tokens):
--   - Created on login
--   - Rotated on every /auth/refresh call
--   - Revoked on logout
--   - All revoked on logout-all or password change
--
-- Password reset tokens:
--   - Single-use, time-limited (default 1 hour)
--   - Marked used_at when consumed
--   - Cannot be reused after used_at is set
-- ============================================================

-- ----------------------------------------------------------
-- Sessions — persisted refresh tokens
-- ----------------------------------------------------------

CREATE TABLE sessions (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL,   -- SHA-256 hex of the raw opaque token
    user_agent TEXT        NOT NULL DEFAULT '',
    ip_address TEXT        NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,            -- NULL = active, non-NULL = revoked
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Primary lookup path: find a session by its token hash
-- (client sends raw token → backend hashes it → looks up here)
CREATE UNIQUE INDEX idx_sessions_token_hash ON sessions (token_hash);

-- Fast revocation: revoke all sessions for a user (logout-all)
CREATE INDEX idx_sessions_user_id ON sessions (user_id);

-- Cleanup: find expired sessions for periodic purge job
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);

COMMENT ON TABLE  sessions            IS 'Persisted refresh token sessions — one row per active device/login';
COMMENT ON COLUMN sessions.token_hash IS 'SHA-256 hex digest of the raw opaque refresh token — never store raw';
COMMENT ON COLUMN sessions.revoked_at IS 'NULL = active session. Set to NOW() on logout or password change';

-- ----------------------------------------------------------
-- Password reset tokens
-- ----------------------------------------------------------

CREATE TABLE password_reset_tokens (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL,   -- SHA-256 hex of the raw reset token
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,            -- NULL = unused, non-NULL = already consumed
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Primary lookup path: find a reset token by hash
CREATE UNIQUE INDEX idx_password_reset_tokens_hash
    ON password_reset_tokens (token_hash);

-- Fast lookup: cancel all reset tokens for a user when password changes
CREATE INDEX idx_password_reset_tokens_user_id
    ON password_reset_tokens (user_id);

COMMENT ON TABLE  password_reset_tokens          IS 'Single-use time-limited password reset tokens';
COMMENT ON COLUMN password_reset_tokens.used_at  IS 'NULL = token not yet used. Set to NOW() on first use — cannot be reused';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS sessions;

-- +goose StatementEnd