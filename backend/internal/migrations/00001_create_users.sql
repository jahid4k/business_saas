-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00001_create_users
-- Creates the core users table.
-- Notes:
--   - id is internal UUID primary key.
--   - public_id is API-facing stable id, e.g. usr_xxx.
--   - email and password_hash are nullable to support OAuth-only users,
--     but your application can still require email for normal SaaS users.
-- ============================================================

CREATE EXTENSION IF NOT EXISTS "pgcrypto"; -- provides gen_random_uuid()

CREATE TABLE users (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id           TEXT        NOT NULL UNIQUE DEFAULT ('usr_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),

    email               TEXT,
    password_hash       TEXT,

    username            TEXT,
    display_name        TEXT        NOT NULL DEFAULT '',
    first_name          TEXT        NOT NULL DEFAULT '',
    last_name           TEXT        NOT NULL DEFAULT '',
    full_name           TEXT        NOT NULL DEFAULT '',

    photo_url           TEXT,
    cover_photo_url     TEXT,

    phone               TEXT,
    phone_verified      BOOLEAN     NOT NULL DEFAULT FALSE,

    email_verified      BOOLEAN     NOT NULL DEFAULT FALSE,
    email_verified_at   TIMESTAMPTZ,

    country             CHAR(2),
    timezone            TEXT        NOT NULL DEFAULT 'UTC',
    locale              TEXT        NOT NULL DEFAULT 'en',
    language            TEXT        NOT NULL DEFAULT 'en',
    currency            TEXT        NOT NULL DEFAULT 'USD',

    status              TEXT        NOT NULL DEFAULT 'active'
                                      CHECK (status IN ('active', 'suspended', 'deleted', 'pending_verification')),
    account_type        TEXT        NOT NULL DEFAULT 'saas_customer',

    suspended_at        TIMESTAMPTZ,
    suspension_reason   TEXT,

    login_redirect_url  TEXT        NOT NULL DEFAULT '/dashboard',
    shortcuts           TEXT[]      NOT NULL DEFAULT ARRAY[]::TEXT[],

    -- Fuse/user UI settings and product preferences
    settings            JSONB       NOT NULL DEFAULT '{}'::JSONB,
    preferences         JSONB       NOT NULL DEFAULT '{}'::JSONB,
    onboarding          JSONB       NOT NULL DEFAULT '{}'::JSONB,
    feature_flags       JSONB       NOT NULL DEFAULT '{}'::JSONB,

    -- Security
    two_fa_enabled      BOOLEAN     NOT NULL DEFAULT FALSE,
    two_fa_secret       TEXT,
    backup_codes        JSONB       NOT NULL DEFAULT '[]'::JSONB,

    last_login_at       TIMESTAMPTZ,
    last_activity_at    TIMESTAMPTZ,
    failed_logins       INTEGER     NOT NULL DEFAULT 0,
    locked_until        TIMESTAMPTZ,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ
);

-- Unique indexes with normalisation.
CREATE UNIQUE INDEX idx_users_email_lower_unique
    ON users (LOWER(email))
    WHERE email IS NOT NULL;

CREATE UNIQUE INDEX idx_users_username_lower_unique
    ON users (LOWER(username))
    WHERE username IS NOT NULL;

CREATE INDEX idx_users_public_id ON users (public_id);
CREATE INDEX idx_users_status ON users (status);
CREATE INDEX idx_users_created_at ON users (created_at);
CREATE INDEX idx_users_last_login_at ON users (last_login_at);

COMMENT ON TABLE  users                     IS 'Registered BusinessSAAS user accounts';
COMMENT ON COLUMN users.id                  IS 'Internal UUID primary key';
COMMENT ON COLUMN users.public_id           IS 'Public API-facing user id, generated with usr_ prefix';
COMMENT ON COLUMN users.email               IS 'Email address; nullable to support OAuth-only or incomplete provider data';
COMMENT ON COLUMN users.password_hash       IS 'Password hash only; never store plaintext';
COMMENT ON COLUMN users.display_name        IS 'User-facing display name';
COMMENT ON COLUMN users.photo_url           IS 'Profile image URL';
COMMENT ON COLUMN users.email_verified      IS 'TRUE after email verification is completed';
COMMENT ON COLUMN users.settings            IS 'FuseSettingsConfigType-compatible user settings JSON';
COMMENT ON COLUMN users.shortcuts           IS 'Fuse app shortcut identifiers';
COMMENT ON COLUMN users.preferences         IS 'User product preferences';
COMMENT ON COLUMN users.onboarding          IS 'Onboarding state and completed steps';
COMMENT ON COLUMN users.feature_flags       IS 'Per-user feature flag overrides';
COMMENT ON COLUMN users.failed_logins       IS 'Consecutive failed login count';
COMMENT ON COLUMN users.locked_until        IS 'Account temporarily locked until this timestamp';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS users;

-- +goose StatementEnd
