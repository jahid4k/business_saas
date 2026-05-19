-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00003_create_permissions
-- Creates canonical permission keys used by roles and authorization.
-- Permission style: resource.action, e.g. billing.manage.
-- ============================================================

CREATE TABLE permissions (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT        NOT NULL UNIQUE DEFAULT ('perm_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),

    key             TEXT        NOT NULL UNIQUE,
    resource        TEXT        NOT NULL,
    action          TEXT        NOT NULL,
    description     TEXT,
    is_system       BOOLEAN     NOT NULL DEFAULT TRUE,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_permissions_resource ON permissions (resource);
CREATE INDEX idx_permissions_action ON permissions (action);

INSERT INTO permissions (key, resource, action, description) VALUES
    ('dashboard.view',       'dashboard',     'view',       'View dashboard'),
    ('organization.view',    'organization',  'view',       'View organization settings'),
    ('organization.update',  'organization',  'update',     'Update organization settings'),
    ('members.view',         'members',       'view',       'View organization members'),
    ('members.invite',       'members',       'invite',     'Invite new members'),
    ('members.update',       'members',       'update',     'Update member roles or metadata'),
    ('members.remove',       'members',       'remove',     'Remove members from organization'),
    ('roles.view',           'roles',         'view',       'View roles'),
    ('roles.assign',         'roles',         'assign',     'Assign roles'),
    ('billing.view',         'billing',       'view',       'View billing and invoices'),
    ('billing.manage',       'billing',       'manage',     'Manage billing and subscriptions'),
    ('subscription.view',    'subscription',  'view',       'View subscription status'),
    ('subscription.update',  'subscription',  'update',     'Update subscription'),
    ('projects.view',        'projects',      'view',       'View projects'),
    ('projects.create',      'projects',      'create',     'Create projects'),
    ('projects.update',      'projects',      'update',     'Update projects'),
    ('projects.delete',      'projects',      'delete',     'Delete projects'),
    ('settings.view',        'settings',      'view',       'View settings'),
    ('settings.update',      'settings',      'update',     'Update settings'),
    ('audit_logs.view',      'audit_logs',    'view',       'View audit logs'),
    ('api_keys.view',        'api_keys',      'view',       'View API keys'),
    ('api_keys.create',      'api_keys',      'create',     'Create API keys'),
    ('api_keys.revoke',      'api_keys',      'revoke',     'Revoke API keys')
ON CONFLICT (key) DO NOTHING;

COMMENT ON TABLE permissions IS 'Canonical authorization permission keys';
COMMENT ON COLUMN permissions.key IS 'Dot-format permission key, e.g. billing.manage';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS permissions;

-- +goose StatementEnd
