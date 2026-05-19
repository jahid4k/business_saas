-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00006_create_auth_accounts
-- Stores linked authentication providers:
-- credentials, google, facebook, github, etc.
-- Similar purpose to Auth.js accounts table, but adapted to app API.
-- ============================================================

CREATE TABLE auth_accounts (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id               TEXT        NOT NULL UNIQUE DEFAULT ('acct_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),

    user_id                 UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    provider                TEXT        NOT NULL,
    provider_account_id     TEXT        NOT NULL,
    provider_type           TEXT        NOT NULL DEFAULT 'oauth'
                                          CHECK (provider_type IN ('oauth', 'oidc', 'credentials', 'email', 'webauthn')),

    access_token            TEXT,
    refresh_token           TEXT,
    id_token                TEXT,
    token_type              TEXT,
    scope                   TEXT,
    expires_at              TIMESTAMPTZ,

    connected_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at            TIMESTAMPTZ,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (provider, provider_account_id)
);

CREATE INDEX idx_auth_accounts_user_id ON auth_accounts (user_id);
CREATE INDEX idx_auth_accounts_provider ON auth_accounts (provider);
CREATE INDEX idx_auth_accounts_provider_account ON auth_accounts (provider, provider_account_id);

COMMENT ON TABLE auth_accounts IS 'Authentication provider accounts linked to application users';
COMMENT ON COLUMN auth_accounts.provider IS 'Provider id, e.g. credentials, google, facebook';
COMMENT ON COLUMN auth_accounts.provider_account_id IS 'Unique user id from the provider';
COMMENT ON COLUMN auth_accounts.access_token IS 'OAuth access token; encrypt at application/storage layer before production use';
COMMENT ON COLUMN auth_accounts.refresh_token IS 'OAuth refresh token; encrypt at application/storage layer before production use';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS auth_accounts;

-- +goose StatementEnd
