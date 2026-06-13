-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00012_add_collaboration_security
-- Adds invitation lifecycle, member deny-overrides, login events,
-- and granular SaaS-grade RBAC/security permissions.
-- ============================================================

ALTER TABLE organization_members
    ADD COLUMN IF NOT EXISTS denied_permissions TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[];

COMMENT ON COLUMN organization_members.denied_permissions IS 'Permission keys explicitly denied for this member. Effective permissions = role + custom - denied.';

CREATE TABLE IF NOT EXISTS organization_invitations (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id           TEXT        NOT NULL UNIQUE DEFAULT ('inv_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),

    org_id              UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email               TEXT        NOT NULL,

    role_id             UUID        REFERENCES roles(id) ON DELETE SET NULL,
    role_key            TEXT        NOT NULL DEFAULT 'member',

    title               TEXT,
    department          TEXT,
    custom_permissions  TEXT[]      NOT NULL DEFAULT ARRAY[]::TEXT[],
    denied_permissions  TEXT[]      NOT NULL DEFAULT ARRAY[]::TEXT[],

    token_hash          TEXT        NOT NULL UNIQUE,
    status              TEXT        NOT NULL DEFAULT 'pending'
                                      CHECK (status IN ('pending', 'accepted', 'revoked', 'expired')),

    invited_by          UUID        REFERENCES users(id) ON DELETE SET NULL,
    accepted_by         UUID        REFERENCES users(id) ON DELETE SET NULL,

    expires_at          TIMESTAMPTZ NOT NULL,
    accepted_at         TIMESTAMPTZ,
    revoked_at          TIMESTAMPTZ,
    last_sent_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resend_count        INTEGER     NOT NULL DEFAULT 0,

    metadata            JSONB       NOT NULL DEFAULT '{}'::JSONB,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_org_invitations_pending_unique
    ON organization_invitations (org_id, LOWER(email))
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_org_invitations_org_id ON organization_invitations (org_id);
CREATE INDEX IF NOT EXISTS idx_org_invitations_email_lower ON organization_invitations (LOWER(email));
CREATE INDEX IF NOT EXISTS idx_org_invitations_token_hash ON organization_invitations (token_hash);
CREATE INDEX IF NOT EXISTS idx_org_invitations_status ON organization_invitations (status);
CREATE INDEX IF NOT EXISTS idx_org_invitations_expires_at ON organization_invitations (expires_at);

CREATE TABLE IF NOT EXISTS login_events (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT        NOT NULL UNIQUE DEFAULT ('login_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),

    user_id         UUID        REFERENCES users(id) ON DELETE SET NULL,
    email           TEXT,
    provider        TEXT        NOT NULL DEFAULT 'credentials',
    status          TEXT        NOT NULL CHECK (status IN ('success', 'failure')),
    failure_reason  TEXT,

    ip_address      INET,
    user_agent      TEXT,
    country         TEXT,
    city            TEXT,
    region          TEXT,

    metadata        JSONB       NOT NULL DEFAULT '{}'::JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_login_events_user_id ON login_events (user_id);
CREATE INDEX IF NOT EXISTS idx_login_events_email_lower ON login_events (LOWER(email)) WHERE email IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_login_events_status ON login_events (status);
CREATE INDEX IF NOT EXISTS idx_login_events_created_at ON login_events (created_at);

INSERT INTO permissions (key, resource, action, description) VALUES
    ('roles.create',                  'roles',       'create',             'Create custom organization roles'),
    ('roles.update',                  'roles',       'update',             'Update custom organization roles'),
    ('roles.delete',                  'roles',       'delete',             'Delete custom organization roles'),
    ('roles.clone',                   'roles',       'clone',              'Clone a system or custom role'),
    ('roles.permissions.update',      'roles',       'permissions.update', 'Update role permission sets'),
    ('members.permissions.view',      'members',     'permissions.view',   'View member permission overrides'),
    ('members.permissions.update',    'members',     'permissions.update', 'Update member permission overrides'),
    ('security.sessions.view',        'security',    'sessions.view',      'View organization sessions'),
    ('security.sessions.revoke',      'security',    'sessions.revoke',    'Revoke organization sessions'),
    ('security.login_events.view',    'security',    'login_events.view',  'View organization login events')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = (
    SELECT ARRAY(
        SELECT DISTINCT p
        FROM UNNEST(permissions || ARRAY[
            'roles.create', 'roles.update', 'roles.delete', 'roles.clone', 'roles.permissions.update',
            'members.permissions.view', 'members.permissions.update',
            'security.sessions.view', 'security.sessions.revoke', 'security.login_events.view'
        ]::TEXT[]) AS p
        ORDER BY p
    )
), updated_at = NOW()
WHERE org_id IS NULL AND name = 'owner';

UPDATE roles
SET permissions = (
    SELECT ARRAY(
        SELECT DISTINCT p
        FROM UNNEST(permissions || ARRAY[
            'roles.create', 'roles.update', 'roles.clone', 'roles.permissions.update',
            'members.permissions.view', 'members.permissions.update',
            'security.sessions.view', 'security.sessions.revoke', 'security.login_events.view'
        ]::TEXT[]) AS p
        ORDER BY p
    )
), updated_at = NOW()
WHERE org_id IS NULL AND name = 'admin';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS login_events;
DROP TABLE IF EXISTS organization_invitations;
ALTER TABLE organization_members DROP COLUMN IF EXISTS denied_permissions;

DELETE FROM permissions WHERE key IN (
    'roles.create', 'roles.update', 'roles.delete', 'roles.clone', 'roles.permissions.update',
    'members.permissions.view', 'members.permissions.update',
    'security.sessions.view', 'security.sessions.revoke', 'security.login_events.view'
);

-- +goose StatementEnd
