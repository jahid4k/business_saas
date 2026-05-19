-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00002_create_organizations
-- Creates SaaS tenant/workspace/company table.
-- ============================================================

CREATE TABLE organizations (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT        NOT NULL UNIQUE DEFAULT ('org_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),

    name            TEXT        NOT NULL,
    slug            TEXT        NOT NULL,
    legal_name      TEXT,

    type            TEXT,
    industry        TEXT,
    website         TEXT,
    logo_url        TEXT,

    country         CHAR(2),
    timezone        TEXT        NOT NULL DEFAULT 'UTC',
    currency        TEXT        NOT NULL DEFAULT 'USD',

    status          TEXT        NOT NULL DEFAULT 'active'
                                  CHECK (status IN ('active', 'suspended', 'deleted')),

    metadata        JSONB       NOT NULL DEFAULT '{}'::JSONB,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_organizations_slug_lower_unique
    ON organizations (LOWER(slug));

CREATE INDEX idx_organizations_public_id ON organizations (public_id);
CREATE INDEX idx_organizations_status ON organizations (status);
CREATE INDEX idx_organizations_created_at ON organizations (created_at);

COMMENT ON TABLE  organizations             IS 'SaaS tenant/workspace/company records';
COMMENT ON COLUMN organizations.id          IS 'Internal UUID primary key';
COMMENT ON COLUMN organizations.public_id   IS 'Public API-facing organization id, generated with org_ prefix';
COMMENT ON COLUMN organizations.name        IS 'Organization display name';
COMMENT ON COLUMN organizations.slug        IS 'Unique workspace slug used in URLs';
COMMENT ON COLUMN organizations.legal_name  IS 'Official/legal business name';
COMMENT ON COLUMN organizations.metadata    IS 'Flexible organization-level metadata';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS organizations;

-- +goose StatementEnd
