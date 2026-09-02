-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00068_platform_notifications
-- Unified notification engine tables (preferences + in-app)
-- ============================================================

CREATE TABLE platform_notification_preferences (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type      TEXT        NOT NULL,
    channel         TEXT        NOT NULL, -- 'email', 'in_app', 'push'
    is_enabled      BOOLEAN     NOT NULL DEFAULT true,
    
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE (user_id, event_type, channel)
);
CREATE INDEX idx_platform_notif_prefs_user_id ON platform_notification_preferences(user_id);

CREATE TABLE platform_notifications (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    event_type      TEXT        NOT NULL,
    channel         TEXT        NOT NULL, -- 'email', 'in_app', 'push'
    
    title           TEXT        NOT NULL,
    body            TEXT        NOT NULL,
    action_url      TEXT,       -- optional deep link
    metadata        JSONB,      -- contextual data for rendering
    
    status          TEXT        NOT NULL DEFAULT 'pending', -- 'pending', 'sent', 'failed'
    error_message   TEXT,
    
    read_at         TIMESTAMPTZ, -- For in_app specifically
    sent_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_platform_notifications_user_channel ON platform_notifications(user_id, channel);
CREATE INDEX idx_platform_notifications_org_id ON platform_notifications(org_id);
CREATE INDEX idx_platform_notifications_created_at ON platform_notifications(created_at);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS platform_notifications;
DROP TABLE IF EXISTS platform_notification_preferences;

-- +goose StatementEnd
