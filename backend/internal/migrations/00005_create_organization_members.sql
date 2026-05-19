-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00005_create_organization_members
-- Creates user-organization membership relationship.
-- This is the core SaaS bridge table.
-- ============================================================

CREATE TABLE organization_members (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id               TEXT        NOT NULL UNIQUE DEFAULT ('mem_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),

    org_id                  UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id                 UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    role_id                 UUID        REFERENCES roles(id) ON DELETE SET NULL,
    role_key                TEXT        NOT NULL DEFAULT 'member',

    title                   TEXT,
    department              TEXT,

    status                  TEXT        NOT NULL DEFAULT 'active'
                                          CHECK (status IN ('active', 'inactive', 'suspended')),

    custom_permissions      TEXT[]      NOT NULL DEFAULT ARRAY[]::TEXT[],

    invitation_status       TEXT        NOT NULL DEFAULT 'accepted'
                                          CHECK (invitation_status IN ('pending', 'accepted', 'rejected', 'expired')),

    invited_by              UUID        REFERENCES users(id) ON DELETE SET NULL,
    invitation_sent_at      TIMESTAMPTZ,
    invitation_accepted_at  TIMESTAMPTZ,
    joined_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (org_id, user_id)
);

CREATE INDEX idx_org_members_org_id ON organization_members (org_id);
CREATE INDEX idx_org_members_user_id ON organization_members (user_id);
CREATE INDEX idx_org_members_role_id ON organization_members (role_id);
CREATE INDEX idx_org_members_role_key ON organization_members (role_key);
CREATE INDEX idx_org_members_status ON organization_members (status);
CREATE INDEX idx_org_members_invitation_status ON organization_members (invitation_status);

COMMENT ON TABLE organization_members IS 'Many-to-many bridge between users and organizations';
COMMENT ON COLUMN organization_members.role_id IS 'Optional FK to roles table';
COMMENT ON COLUMN organization_members.role_key IS 'Role key snapshot used by API/session, e.g. owner/admin/member';
COMMENT ON COLUMN organization_members.custom_permissions IS 'Extra permission keys granted directly to this member';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS organization_members;

-- +goose StatementEnd
