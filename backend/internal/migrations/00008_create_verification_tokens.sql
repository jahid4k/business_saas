-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00008_create_verification_tokens
-- Stores one-time verification and recovery tokens.
-- Raw tokens must never be stored; store a hash only.
-- ============================================================

CREATE TABLE verification_tokens (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT        NOT NULL UNIQUE DEFAULT ('vt_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),

    user_id         UUID        REFERENCES users(id) ON DELETE CASCADE,
    email           TEXT,

    token_hash      TEXT        NOT NULL UNIQUE,

    type            TEXT        NOT NULL
                                  CHECK (type IN (
                                      'email_verification',
                                      'password_reset',
                                      'magic_link',
                                      'invitation',
                                      'two_factor'
                                  )),

    verified_at     TIMESTAMPTZ,
    used_at         TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_verification_tokens_user_id ON verification_tokens (user_id);
CREATE INDEX idx_verification_tokens_email_lower ON verification_tokens (LOWER(email)) WHERE email IS NOT NULL;
CREATE INDEX idx_verification_tokens_type ON verification_tokens (type);
CREATE INDEX idx_verification_tokens_expires_at ON verification_tokens (expires_at);

COMMENT ON TABLE verification_tokens IS 'Email verification, password reset, magic link, invitation, and 2FA one-time tokens';
COMMENT ON COLUMN verification_tokens.token_hash IS 'Hash of one-time token; never store raw token';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS verification_tokens;

-- +goose StatementEnd
