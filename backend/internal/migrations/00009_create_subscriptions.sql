-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00009_create_subscriptions
-- Stores organization billing/subscription status.
-- In B2B SaaS, subscription usually belongs to organization.
-- ============================================================

CREATE TABLE subscriptions (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id                   TEXT        NOT NULL UNIQUE DEFAULT ('sub_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),

    org_id                      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    plan                        TEXT        NOT NULL DEFAULT 'free'
                                              CHECK (plan IN ('free', 'pro', 'business', 'enterprise')),
    plan_name                   TEXT        NOT NULL DEFAULT 'Free',

    status                      TEXT        NOT NULL DEFAULT 'active'
                                              CHECK (status IN ('trialing', 'active', 'past_due', 'cancelled', 'expired')),

    billing_cycle               TEXT        CHECK (billing_cycle IN ('monthly', 'yearly', 'lifetime')),

    currency                    TEXT        NOT NULL DEFAULT 'USD',
    amount                      NUMERIC(12, 2) NOT NULL DEFAULT 0,

    trial_started_at            TIMESTAMPTZ,
    trial_ends_at               TIMESTAMPTZ,

    current_period_start        TIMESTAMPTZ,
    current_period_end          TIMESTAMPTZ,

    cancel_at_period_end        BOOLEAN     NOT NULL DEFAULT FALSE,

    payment_provider            TEXT,
    payment_customer_id         TEXT,
    payment_subscription_id     TEXT,

    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscriptions_org_id ON subscriptions (org_id);
CREATE INDEX idx_subscriptions_status ON subscriptions (status);
CREATE INDEX idx_subscriptions_plan ON subscriptions (plan);
CREATE INDEX idx_subscriptions_current_period_end ON subscriptions (current_period_end);
CREATE UNIQUE INDEX idx_subscriptions_payment_subscription_id_unique
    ON subscriptions (payment_provider, payment_subscription_id)
    WHERE payment_provider IS NOT NULL AND payment_subscription_id IS NOT NULL;

COMMENT ON TABLE subscriptions IS 'Organization subscription and billing state';
COMMENT ON COLUMN subscriptions.org_id IS 'Subscription owner organization';
COMMENT ON COLUMN subscriptions.payment_customer_id IS 'External billing provider customer id';
COMMENT ON COLUMN subscriptions.payment_subscription_id IS 'External billing provider subscription id';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS subscriptions;

-- +goose StatementEnd
