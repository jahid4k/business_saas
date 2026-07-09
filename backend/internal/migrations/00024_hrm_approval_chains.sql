-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00022_hrm_approval_chains
--
-- HRM configurable approval engine (Group A2):
--   hrm_approval_templates        — HR-defined approval chain definitions
--   hrm_approval_template_levels  — ordered levels within a template
--   hrm_approval_instances        — runtime instance for a specific entity
--   hrm_approval_decisions        — per-level decisions on an instance
--
-- Design notes:
--   • Templates are defined per action_type (leave, promotion, transfer, etc.)
--   • Multiple templates per action_type are supported; condition_expression
--     selects which one applies (e.g. "leave_days > 10 → extended template").
--   • On instance creation, the full level chain is snapshotted into
--     instance_snapshot JSONB — template changes do NOT affect live instances.
--   • approver_type drives who must act:
--       reporting_manager → employee.manager_id
--       dept_head         → department.head_employee_id
--       role              → any org member with approver_role
--       specific_user     → exactly approver_user_id
--   • SLA breach action: escalate_next | auto_approve | auto_reject
-- ============================================================

-- ------------------------------------------------------------
-- hrm_approval_templates
-- ------------------------------------------------------------
CREATE TABLE hrm_approval_templates (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id            TEXT        NOT NULL UNIQUE
                                         DEFAULT ('apt_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id               UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name                 TEXT        NOT NULL,
    description          TEXT,
    action_type          TEXT        NOT NULL
                                         CHECK (action_type IN (
                                             'leave', 'resignation', 'promotion', 'transfer',
                                             'warning', 'document', 'termination',
                                             'attendance_regularization', 'custom'
                                         )),

    -- Optional condition expression (expr-lang); evaluated with entity fields as env
    -- NULL means "always use this template" (default for the action_type)
    condition_expression TEXT,

    is_default           BOOLEAN     NOT NULL DEFAULT FALSE,
    is_active            BOOLEAN     NOT NULL DEFAULT TRUE,

    created_by           UUID        NOT NULL REFERENCES users(id),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_apt_org_id      ON hrm_approval_templates (org_id);
CREATE INDEX idx_hrm_apt_action_type ON hrm_approval_templates (org_id, action_type);

-- Only one default per org+action_type
CREATE UNIQUE INDEX idx_hrm_apt_default
    ON hrm_approval_templates (org_id, action_type)
    WHERE is_default = TRUE AND is_active = TRUE;

COMMENT ON TABLE  hrm_approval_templates IS 'HR-defined approval chain configurations per action type';
COMMENT ON COLUMN hrm_approval_templates.condition_expression IS 'Optional expr-lang condition selecting this template; NULL = always apply';
COMMENT ON COLUMN hrm_approval_templates.is_default           IS 'Exactly one default per org+action_type; used when no condition matches';

-- ------------------------------------------------------------
-- hrm_approval_template_levels
-- ------------------------------------------------------------
CREATE TABLE hrm_approval_template_levels (
    id               UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id      UUID    NOT NULL REFERENCES hrm_approval_templates(id) ON DELETE CASCADE,
    level            INTEGER NOT NULL CHECK (level >= 1),

    approver_type    TEXT    NOT NULL
                                 CHECK (approver_type IN
                                     ('reporting_manager', 'dept_head', 'role', 'specific_user')),

    -- Used when approver_type = 'role'
    approver_role    TEXT,
    -- Used when approver_type = 'specific_user'
    approver_user_id UUID    REFERENCES users(id) ON DELETE SET NULL,

    sla_hours        INTEGER NOT NULL DEFAULT 48 CHECK (sla_hours > 0),
    on_sla_breach    TEXT    NOT NULL DEFAULT 'escalate_next'
                                 CHECK (on_sla_breach IN ('escalate_next', 'auto_approve', 'auto_reject')),

    CONSTRAINT uq_hrm_atl_level UNIQUE (template_id, level)
);

CREATE INDEX idx_hrm_atl_template_id ON hrm_approval_template_levels (template_id);

COMMENT ON TABLE  hrm_approval_template_levels IS 'Ordered levels within an approval template';
COMMENT ON COLUMN hrm_approval_template_levels.on_sla_breach IS 'What happens when SLA expires without action';

-- ------------------------------------------------------------
-- hrm_approval_instances
-- ------------------------------------------------------------
CREATE TABLE hrm_approval_instances (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id           TEXT        NOT NULL UNIQUE
                                        DEFAULT ('api_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id              UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    template_id         UUID        REFERENCES hrm_approval_templates(id) ON DELETE SET NULL,

    -- Polymorphic: what entity this instance is for
    entity_type         TEXT        NOT NULL
                                        CHECK (entity_type IN (
                                            'leave_request', 'resignation', 'promotion', 'transfer',
                                            'warning', 'document', 'termination',
                                            'attendance_regularization', 'custom'
                                        )),
    entity_id           UUID        NOT NULL,

    -- Frozen copy of the level chain at the time of creation
    -- Ensures template edits don't affect live instances
    instance_snapshot   JSONB       NOT NULL DEFAULT '[]',

    current_level       INTEGER     NOT NULL DEFAULT 1,
    overall_status      TEXT        NOT NULL DEFAULT 'pending'
                                        CHECK (overall_status IN
                                            ('pending', 'approved', 'rejected', 'cancelled')),

    requested_by        UUID        NOT NULL REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at        TIMESTAMPTZ
);

CREATE INDEX idx_hrm_api_org_id      ON hrm_approval_instances (org_id);
CREATE INDEX idx_hrm_api_entity      ON hrm_approval_instances (entity_type, entity_id);
CREATE INDEX idx_hrm_api_status      ON hrm_approval_instances (overall_status);
CREATE INDEX idx_hrm_api_template_id ON hrm_approval_instances (template_id) WHERE template_id IS NOT NULL;

COMMENT ON TABLE  hrm_approval_instances IS 'Runtime approval instance for a specific entity';
COMMENT ON COLUMN hrm_approval_instances.instance_snapshot IS 'Frozen level chain at creation time; immune to template edits';
COMMENT ON COLUMN hrm_approval_instances.entity_id         IS 'UUID of the entity being approved (leave_request, promotion, etc.)';

-- ------------------------------------------------------------
-- hrm_approval_decisions
-- ------------------------------------------------------------
CREATE TABLE hrm_approval_decisions (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID        NOT NULL REFERENCES hrm_approval_instances(id) ON DELETE CASCADE,
    level       INTEGER     NOT NULL CHECK (level >= 1),

    approver_id UUID        NOT NULL REFERENCES users(id),
    action      TEXT        NOT NULL CHECK (action IN ('approved', 'rejected', 'cancelled')),
    note        TEXT,
    decided_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_hrm_ad_instance_level UNIQUE (instance_id, level)
);

CREATE INDEX idx_hrm_ad_instance_id ON hrm_approval_decisions (instance_id);
CREATE INDEX idx_hrm_ad_approver_id ON hrm_approval_decisions (approver_id);

COMMENT ON TABLE hrm_approval_decisions IS 'Per-level decision records on an approval instance';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_approval_decisions;
DROP TABLE IF EXISTS hrm_approval_instances;
DROP TABLE IF EXISTS hrm_approval_template_levels;
DROP TABLE IF EXISTS hrm_approval_templates;

-- +goose StatementEnd
