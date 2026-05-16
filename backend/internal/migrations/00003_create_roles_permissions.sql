-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00003_create_roles_permissions
-- Creates roles, permissions, and role_permissions tables.
-- Seeds the four system roles and all Phase 1 permissions.
--
-- NOTE: memberships live in 00004 because they depend on both
-- roles (this migration) and businesses (00002).
-- ============================================================

-- ----------------------------------------------------------
-- Roles
-- System roles are seeded here. They are not user-editable.
-- ----------------------------------------------------------

CREATE TABLE roles (
    id          UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT    NOT NULL,
    description TEXT    NOT NULL DEFAULT '',
    is_system   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_roles_name ON roles (name);

COMMENT ON TABLE  roles           IS 'System-defined roles — owner, admin, member, viewer';
COMMENT ON COLUMN roles.is_system IS 'TRUE for seeded system roles that cannot be deleted';

-- ----------------------------------------------------------
-- Permissions
-- Granular capabilities in the format: resource.action
-- ----------------------------------------------------------

CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource    TEXT NOT NULL,
    action      TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Composite unique: each resource+action pair is defined once
CREATE UNIQUE INDEX idx_permissions_resource_action ON permissions (resource, action);

COMMENT ON TABLE  permissions          IS 'Granular permission definitions e.g. tasks.delete';
COMMENT ON COLUMN permissions.resource IS 'The resource being protected, e.g. tasks, members, business';
COMMENT ON COLUMN permissions.action   IS 'The action being controlled, e.g. read, create, update, delete';

-- ----------------------------------------------------------
-- Role ↔ Permission junction
-- ----------------------------------------------------------

CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE INDEX idx_role_permissions_role_id ON role_permissions (role_id);

COMMENT ON TABLE role_permissions IS 'Which permissions each role grants';

-- ----------------------------------------------------------
-- Seed: system roles
-- ----------------------------------------------------------

INSERT INTO roles (id, name, description, is_system) VALUES
    ('00000000-0000-0000-0000-000000000001', 'owner',  'Full access to everything including business settings', TRUE),
    ('00000000-0000-0000-0000-000000000002', 'admin',  'Full access except business settings',                 TRUE),
    ('00000000-0000-0000-0000-000000000003', 'member', 'Can read, create and update tasks',                    TRUE),
    ('00000000-0000-0000-0000-000000000004', 'viewer', 'Read-only access to tasks',                            TRUE);

-- ----------------------------------------------------------
-- Seed: permissions (Phase 1 — task module + member/business management)
-- ----------------------------------------------------------

INSERT INTO permissions (id, resource, action, description) VALUES
    ('00000000-0000-0001-0000-000000000001', 'tasks',    'read',   'View tasks in the workspace'),
    ('00000000-0000-0001-0000-000000000002', 'tasks',    'create', 'Create new tasks'),
    ('00000000-0000-0001-0000-000000000003', 'tasks',    'update', 'Edit existing tasks'),
    ('00000000-0000-0001-0000-000000000004', 'tasks',    'delete', 'Delete tasks'),
    ('00000000-0000-0001-0000-000000000005', 'members',  'manage', 'Invite members and assign roles'),
    ('00000000-0000-0001-0000-000000000006', 'business', 'manage', 'Edit workspace name, slug and settings');

-- ----------------------------------------------------------
-- Seed: role → permission assignments
--
-- Owner:  tasks.read, tasks.create, tasks.update, tasks.delete,
--         members.manage, business.manage
-- Admin:  tasks.read, tasks.create, tasks.update, tasks.delete,
--         members.manage
-- Member: tasks.read, tasks.create, tasks.update
-- Viewer: tasks.read
-- ----------------------------------------------------------

-- Owner — all permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000001', id FROM permissions;

-- Admin — all except business.manage
INSERT INTO role_permissions (role_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000002', id
FROM permissions
WHERE NOT (resource = 'business' AND action = 'manage');

-- Member — tasks only (read, create, update)
INSERT INTO role_permissions (role_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000003', id
FROM permissions
WHERE resource = 'tasks' AND action IN ('read', 'create', 'update');

-- Viewer — tasks read only
INSERT INTO role_permissions (role_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000004', id
FROM permissions
WHERE resource = 'tasks' AND action = 'read';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Remove in reverse dependency order
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;

-- +goose StatementEnd