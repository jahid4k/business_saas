-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00010_create_organization_usage
-- Stores per-period usage and limits for each organization.
-- JSONB is flexible for early SaaS; later this can be normalized.
-- ============================================================

CREATE TABLE organization_usage (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id           TEXT        NOT NULL UNIQUE DEFAULT ('usage_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),

    org_id              UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    subscription_id     UUID        REFERENCES subscriptions(id) ON DELETE SET NULL,

    period_start        TIMESTAMPTZ NOT NULL,
    period_end          TIMESTAMPTZ NOT NULL,

    limits              JSONB       NOT NULL DEFAULT '{}'::JSONB,
    used                JSONB       NOT NULL DEFAULT '{}'::JSONB,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (org_id, period_start, period_end)
);

CREATE INDEX idx_organization_usage_org_id ON organization_usage (org_id);
CREATE INDEX idx_organization_usage_subscription_id ON organization_usage (subscription_id);
CREATE INDEX idx_organization_usage_period ON organization_usage (period_start, period_end);

COMMENT ON TABLE organization_usage IS 'Organization usage counters and plan limits per billing period';
COMMENT ON COLUMN organization_usage.limits IS 'JSON limits object, e.g. members/projects/storageGB/apiRequestsPerMonth';
COMMENT ON COLUMN organization_usage.used IS 'JSON usage object, e.g. used projects/storage/api requests';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS organization_usage;

-- +goose StatementEnd
