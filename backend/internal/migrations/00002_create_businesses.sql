-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00002_create_businesses
-- Creates the businesses (workspaces/tenants) table.
-- Every piece of business data is scoped to business_id.
-- ============================================================

CREATE TABLE businesses (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    slug       TEXT        NOT NULL,
    owner_id   UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    is_active  BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Slug must be globally unique — used in URLs and API calls
CREATE UNIQUE INDEX idx_businesses_slug ON businesses (slug);

-- Lookup businesses by owner (used in "my workspaces" queries)
CREATE INDEX idx_businesses_owner_id ON businesses (owner_id);

COMMENT ON TABLE  businesses           IS 'Workspaces / tenants in BusinessSAAS';
COMMENT ON COLUMN businesses.slug      IS 'URL-safe unique identifier, e.g. acme-corp';
COMMENT ON COLUMN businesses.owner_id  IS 'The user who created this business — always has Owner role';
COMMENT ON COLUMN businesses.is_active IS 'FALSE for suspended or deleted businesses';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS businesses;

-- +goose StatementEnd