-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00001_create_users
-- Creates the core users table.
-- ============================================================

CREATE EXTENSION IF NOT EXISTS "pgcrypto"; -- provides gen_random_uuid()

CREATE TABLE users (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT        NOT NULL,
    password_hash   TEXT        NOT NULL,
    first_name      TEXT        NOT NULL DEFAULT '',
    last_name       TEXT        NOT NULL DEFAULT '',
    is_verified     BOOLEAN     NOT NULL DEFAULT FALSE,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    failed_logins   INTEGER     NOT NULL DEFAULT 0,
    locked_until    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Unique index on email (normalised to lowercase)
-- We normalise email in the application before insert,
-- so this index ensures uniqueness at the DB level too.
CREATE UNIQUE INDEX idx_users_email ON users (LOWER(email));

-- Index for lookup by ID (covered by PK, listed for documentation)
-- CREATE INDEX idx_users_id ON users (id); -- already covered by PK

COMMENT ON TABLE  users               IS 'Registered BusinessSAAS user accounts';
COMMENT ON COLUMN users.id            IS 'UUID primary key';
COMMENT ON COLUMN users.email         IS 'Unique email address (application normalises to lowercase before insert)';
COMMENT ON COLUMN users.password_hash IS 'bcrypt hash — never store plaintext';
COMMENT ON COLUMN users.is_verified   IS 'TRUE after email verification is completed';
COMMENT ON COLUMN users.is_active     IS 'FALSE for deactivated/banned accounts';
COMMENT ON COLUMN users.failed_logins IS 'Count of consecutive failed login attempts';
COMMENT ON COLUMN users.locked_until  IS 'Account temporarily locked until this time after repeated failures';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS users;

-- +goose StatementEnd