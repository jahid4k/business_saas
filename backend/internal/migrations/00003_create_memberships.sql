-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00004_create_memberships
-- Creates the memberships table — the core multi-tenancy join.
--
-- A membership connects one user to one business with one role.
-- One user can belong to many businesses (different memberships).
-- One user can have at most ONE role per business.
--
-- This is the table that drives every authorization check:
--   user → membership → role → role_permissions → permission
-- ============================================================

CREATE TABLE memberships (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id)      ON DELETE CASCADE,
    business_id UUID        NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    role_id     UUID        NOT NULL REFERENCES roles(id)      ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- A user can only have ONE membership per business
CREATE UNIQUE INDEX idx_memberships_user_business
    ON memberships (user_id, business_id);

-- Fast lookup: all members of a business
CREATE INDEX idx_memberships_business_id ON memberships (business_id);

-- Fast lookup: all businesses a user belongs to
CREATE INDEX idx_memberships_user_id ON memberships (user_id);

COMMENT ON TABLE  memberships            IS 'Connects users to businesses with a specific role';
COMMENT ON COLUMN memberships.user_id     IS 'The member user';
COMMENT ON COLUMN memberships.business_id IS 'The workspace they belong to';
COMMENT ON COLUMN memberships.role_id     IS 'Their role within this workspace';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS memberships;

-- +goose StatementEnd