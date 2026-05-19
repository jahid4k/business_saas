-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00004_create_roles
-- Creates system and organization-specific roles.
-- org_id NULL means a system/global role template.
-- ============================================================

CREATE TABLE roles (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT        NOT NULL UNIQUE DEFAULT ('role_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),

    org_id          UUID        REFERENCES organizations(id) ON DELETE CASCADE,

    name            TEXT        NOT NULL,
    description     TEXT,
    permissions     TEXT[]      NOT NULL DEFAULT ARRAY[]::TEXT[],

    is_system       BOOLEAN     NOT NULL DEFAULT FALSE,
    is_custom       BOOLEAN     NOT NULL DEFAULT FALSE,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- For organization-specific roles.
CREATE UNIQUE INDEX idx_roles_org_name_unique
    ON roles (org_id, LOWER(name))
    WHERE org_id IS NOT NULL;

-- For system/global role templates.
CREATE UNIQUE INDEX idx_roles_system_name_unique
    ON roles (LOWER(name))
    WHERE org_id IS NULL;

CREATE INDEX idx_roles_org_id ON roles (org_id);
CREATE INDEX idx_roles_is_system ON roles (is_system);

INSERT INTO roles (org_id, name, description, permissions, is_system, is_custom) VALUES
    (
        NULL,
        'owner',
        'Full organization owner with all permissions',
        ARRAY[
            'dashboard.view',
            'organization.view', 'organization.update',
            'members.view', 'members.invite', 'members.update', 'members.remove',
            'roles.view', 'roles.assign',
            'billing.view', 'billing.manage',
            'subscription.view', 'subscription.update',
            'projects.view', 'projects.create', 'projects.update', 'projects.delete',
            'settings.view', 'settings.update',
            'audit_logs.view',
            'api_keys.view', 'api_keys.create', 'api_keys.revoke'
        ],
        TRUE,
        FALSE
    ),
    (
        NULL,
        'admin',
        'Organization admin with broad management access',
        ARRAY[
            'dashboard.view',
            'organization.view', 'organization.update',
            'members.view', 'members.invite', 'members.update',
            'roles.view', 'roles.assign',
            'billing.view',
            'subscription.view',
            'projects.view', 'projects.create', 'projects.update', 'projects.delete',
            'settings.view', 'settings.update',
            'audit_logs.view',
            'api_keys.view', 'api_keys.create', 'api_keys.revoke'
        ],
        TRUE,
        FALSE
    ),
    (
        NULL,
        'manager',
        'Manager with project and member visibility',
        ARRAY[
            'dashboard.view',
            'organization.view',
            'members.view',
            'projects.view', 'projects.create', 'projects.update',
            'settings.view'
        ],
        TRUE,
        FALSE
    ),
    (
        NULL,
        'member',
        'Regular organization member',
        ARRAY[
            'dashboard.view',
            'organization.view',
            'projects.view', 'projects.create', 'projects.update',
            'settings.view', 'settings.update'
        ],
        TRUE,
        FALSE
    ),
    (
        NULL,
        'viewer',
        'Read-only organization viewer',
        ARRAY[
            'dashboard.view',
            'organization.view',
            'members.view',
            'projects.view',
            'settings.view'
        ],
        TRUE,
        FALSE
    )
ON CONFLICT DO NOTHING;

COMMENT ON TABLE roles IS 'System and tenant-specific roles';
COMMENT ON COLUMN roles.org_id IS 'NULL for global system role templates; organization id for tenant-specific custom roles';
COMMENT ON COLUMN roles.permissions IS 'Array of permission keys, e.g. billing.manage';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS roles;

-- +goose StatementEnd
