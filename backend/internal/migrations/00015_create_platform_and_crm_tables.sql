-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00013_create_platform_and_crm_tables
--
-- Platform tables (shared across all future modules):
--   platform_contacts    — people records
--   platform_companies   — company/org records
--   platform_notes       — notes on any entity, any module
--   platform_tasks       — tasks on any entity, any module
--   platform_activities  — activity log on any entity, any module
--   platform_email_logs  — email records on any entity, any module
--
-- CRM-specific tables:
--   crm_leads            — sales leads
--   crm_pipelines        — deal pipelines
--   crm_pipeline_stages  — stages within a pipeline
--   crm_deals            — deals
--
-- The module TEXT column on platform engagement tables records which
-- module created the record (e.g. 'crm', 'erp', 'hrm').
-- This enables per-module filtering and unified cross-module timelines.
-- ============================================================

-- ------------------------------------------------------------
-- platform_contacts
-- ------------------------------------------------------------
CREATE TABLE platform_contacts (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE DEFAULT ('con_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    first_name  TEXT        NOT NULL,
    last_name   TEXT,
    email       TEXT,
    phone       TEXT,
    title       TEXT,
    company_id  UUID,       -- FK added below after platform_companies exists
    source      TEXT,
    status      TEXT        NOT NULL DEFAULT 'active'
                                CHECK (status IN ('active', 'inactive', 'archived')),
    owner_id    UUID        REFERENCES users(id) ON DELETE SET NULL,

    created_by  UUID        NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_pcon_org_id     ON platform_contacts (org_id);
CREATE INDEX idx_pcon_email      ON platform_contacts (LOWER(email)) WHERE email IS NOT NULL;
CREATE INDEX idx_pcon_company_id ON platform_contacts (company_id);
CREATE INDEX idx_pcon_deleted_at ON platform_contacts (deleted_at);

COMMENT ON TABLE platform_contacts IS 'Shared people records used by CRM, HRM, ERP and any future module';

-- ------------------------------------------------------------
-- platform_companies
-- ------------------------------------------------------------
CREATE TABLE platform_companies (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE DEFAULT ('cmp_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name        TEXT        NOT NULL,
    domain      TEXT,
    industry    TEXT,
    website     TEXT,
    phone       TEXT,
    address     TEXT,
    country     TEXT,
    status      TEXT        NOT NULL DEFAULT 'active'
                                CHECK (status IN ('active', 'inactive', 'archived')),
    owner_id    UUID        REFERENCES users(id) ON DELETE SET NULL,

    created_by  UUID        NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_pcmp_org_id     ON platform_companies (org_id);
CREATE INDEX idx_pcmp_deleted_at ON platform_companies (deleted_at);

COMMENT ON TABLE platform_companies IS 'Shared company records used by CRM, ERP, Accounting and any future module';

-- Add FK from contacts to companies now that both tables exist
ALTER TABLE platform_contacts
    ADD CONSTRAINT fk_pcon_company
    FOREIGN KEY (company_id) REFERENCES platform_companies(id) ON DELETE SET NULL;

-- ------------------------------------------------------------
-- platform_notes
-- ------------------------------------------------------------
CREATE TABLE platform_notes (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id    TEXT        NOT NULL UNIQUE DEFAULT ('note_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id       UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    module       TEXT        NOT NULL DEFAULT 'crm',  -- 'crm' | 'erp' | 'hrm' etc.
    content      TEXT        NOT NULL,
    related_type TEXT,       -- 'platform.contact' | 'crm.deal' | 'erp.purchase_order' etc.
    related_id   UUID,
    created_by   UUID        NOT NULL REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pnote_org_id  ON platform_notes (org_id);
CREATE INDEX idx_pnote_module  ON platform_notes (module);
CREATE INDEX idx_pnote_related ON platform_notes (related_type, related_id);

COMMENT ON TABLE platform_notes IS 'Shared notes for any entity across all modules';
COMMENT ON COLUMN platform_notes.module IS 'Module that created this note: crm, erp, hrm, etc.';

-- ------------------------------------------------------------
-- platform_tasks
-- ------------------------------------------------------------
CREATE TABLE platform_tasks (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id    TEXT        NOT NULL UNIQUE DEFAULT ('ptsk_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id       UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    module       TEXT        NOT NULL DEFAULT 'crm',
    title        TEXT        NOT NULL,
    description  TEXT,
    due_date     TIMESTAMPTZ,
    status       TEXT        NOT NULL DEFAULT 'open'
                                 CHECK (status IN ('open', 'completed')),
    priority     TEXT        NOT NULL DEFAULT 'medium'
                                 CHECK (priority IN ('low', 'medium', 'high')),
    related_type TEXT,
    related_id   UUID,
    assigned_to  UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_by   UUID        NOT NULL REFERENCES users(id),
    completed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ptask_org_id      ON platform_tasks (org_id);
CREATE INDEX idx_ptask_module      ON platform_tasks (module);
CREATE INDEX idx_ptask_related     ON platform_tasks (related_type, related_id);
CREATE INDEX idx_ptask_assigned_to ON platform_tasks (assigned_to);
CREATE INDEX idx_ptask_status      ON platform_tasks (status);
CREATE INDEX idx_ptask_due_date    ON platform_tasks (due_date);

COMMENT ON TABLE platform_tasks IS 'Shared tasks for any entity across all modules';

-- ------------------------------------------------------------
-- platform_activities
-- ------------------------------------------------------------
CREATE TABLE platform_activities (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id     TEXT        NOT NULL UNIQUE DEFAULT ('act_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id        UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    module        TEXT        NOT NULL DEFAULT 'crm',
    type          TEXT        NOT NULL
                                  CHECK (type IN ('call', 'email', 'meeting', 'note', 'task', 'other')),
    subject       TEXT        NOT NULL,
    description   TEXT,
    outcome       TEXT,
    related_type  TEXT,
    related_id    UUID,
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    duration_mins INTEGER,
    created_by    UUID        NOT NULL REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pact_org_id  ON platform_activities (org_id);
CREATE INDEX idx_pact_module  ON platform_activities (module);
CREATE INDEX idx_pact_related ON platform_activities (related_type, related_id);
CREATE INDEX idx_pact_type    ON platform_activities (type);

COMMENT ON TABLE platform_activities IS 'Shared activity log for any entity across all modules';

-- ------------------------------------------------------------
-- platform_email_logs
-- ------------------------------------------------------------
CREATE TABLE platform_email_logs (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id    TEXT        NOT NULL UNIQUE DEFAULT ('eml_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id       UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    module       TEXT        NOT NULL DEFAULT 'crm',
    subject      TEXT        NOT NULL,
    body         TEXT,
    from_email   TEXT        NOT NULL,
    to_email     TEXT        NOT NULL,
    direction    TEXT        NOT NULL DEFAULT 'outbound'
                                 CHECK (direction IN ('inbound', 'outbound')),
    status       TEXT        NOT NULL DEFAULT 'sent'
                                 CHECK (status IN ('sent', 'received', 'draft', 'failed')),
    related_type TEXT,
    related_id   UUID,
    sent_at      TIMESTAMPTZ,
    created_by   UUID        NOT NULL REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_peml_org_id  ON platform_email_logs (org_id);
CREATE INDEX idx_peml_module  ON platform_email_logs (module);
CREATE INDEX idx_peml_related ON platform_email_logs (related_type, related_id);

COMMENT ON TABLE platform_email_logs IS 'Shared email log for any entity across all modules';

-- ------------------------------------------------------------
-- crm_leads
-- ------------------------------------------------------------
CREATE TABLE crm_leads (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id            TEXT        NOT NULL UNIQUE DEFAULT ('ld_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id               UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    first_name           TEXT        NOT NULL,
    last_name            TEXT,
    email                TEXT,
    phone                TEXT,
    company_name         TEXT,
    title                TEXT,
    source               TEXT,
    status               TEXT        NOT NULL DEFAULT 'new'
                                         CHECK (status IN ('new', 'contacted', 'qualified', 'unqualified', 'converted')),

    converted_at         TIMESTAMPTZ,
    converted_contact_id UUID        REFERENCES platform_contacts(id) ON DELETE SET NULL,
    converted_deal_id    UUID,       -- FK added after crm_deals exists

    owner_id             UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_by           UUID        NOT NULL REFERENCES users(id),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at           TIMESTAMPTZ
);

CREATE INDEX idx_crm_leads_org_id     ON crm_leads (org_id);
CREATE INDEX idx_crm_leads_status     ON crm_leads (status);
CREATE INDEX idx_crm_leads_owner_id   ON crm_leads (owner_id);
CREATE INDEX idx_crm_leads_deleted_at ON crm_leads (deleted_at);

COMMENT ON TABLE crm_leads IS 'CRM leads — pre-conversion sales prospects';

-- ------------------------------------------------------------
-- crm_pipelines
-- ------------------------------------------------------------
CREATE TABLE crm_pipelines (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE DEFAULT ('pip_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name        TEXT        NOT NULL,
    description TEXT,
    is_default  BOOLEAN     NOT NULL DEFAULT FALSE,

    created_by  UUID        NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_crm_pipelines_org_id ON crm_pipelines (org_id);
CREATE UNIQUE INDEX idx_crm_pipelines_org_name ON crm_pipelines (org_id, LOWER(name));

COMMENT ON TABLE crm_pipelines IS 'CRM deal pipelines scoped to an organization';

-- ------------------------------------------------------------
-- crm_pipeline_stages
-- ------------------------------------------------------------
CREATE TABLE crm_pipeline_stages (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE DEFAULT ('stg_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    pipeline_id UUID        NOT NULL REFERENCES crm_pipelines(id) ON DELETE CASCADE,

    name        TEXT        NOT NULL,
    position    INTEGER     NOT NULL DEFAULT 0,
    probability INTEGER     NOT NULL DEFAULT 0 CHECK (probability BETWEEN 0 AND 100),

    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_crm_stages_pipeline_id ON crm_pipeline_stages (pipeline_id);
CREATE INDEX idx_crm_stages_org_id      ON crm_pipeline_stages (org_id);
CREATE UNIQUE INDEX idx_crm_stages_pipeline_name ON crm_pipeline_stages (pipeline_id, LOWER(name));

COMMENT ON TABLE crm_pipeline_stages IS 'Ordered stages within a CRM pipeline';

-- ------------------------------------------------------------
-- crm_deals
-- ------------------------------------------------------------
CREATE TABLE crm_deals (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE DEFAULT ('deal_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    title       TEXT        NOT NULL,
    value       NUMERIC(15, 2) NOT NULL DEFAULT 0,
    currency    TEXT        NOT NULL DEFAULT 'USD',

    pipeline_id UUID        NOT NULL REFERENCES crm_pipelines(id) ON DELETE RESTRICT,
    stage_id    UUID        NOT NULL REFERENCES crm_pipeline_stages(id) ON DELETE RESTRICT,

    contact_id  UUID        REFERENCES platform_contacts(id) ON DELETE SET NULL,
    company_id  UUID        REFERENCES platform_companies(id) ON DELETE SET NULL,

    status      TEXT        NOT NULL DEFAULT 'open'
                                CHECK (status IN ('open', 'won', 'lost')),
    close_date  DATE,
    lost_reason TEXT,
    owner_id    UUID        REFERENCES users(id) ON DELETE SET NULL,

    won_at      TIMESTAMPTZ,
    lost_at     TIMESTAMPTZ,

    created_by  UUID        NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_crm_deals_org_id      ON crm_deals (org_id);
CREATE INDEX idx_crm_deals_pipeline_id ON crm_deals (pipeline_id);
CREATE INDEX idx_crm_deals_stage_id    ON crm_deals (stage_id);
CREATE INDEX idx_crm_deals_contact_id  ON crm_deals (contact_id);
CREATE INDEX idx_crm_deals_company_id  ON crm_deals (company_id);
CREATE INDEX idx_crm_deals_owner_id    ON crm_deals (owner_id);
CREATE INDEX idx_crm_deals_status      ON crm_deals (status);
CREATE INDEX idx_crm_deals_deleted_at  ON crm_deals (deleted_at);

COMMENT ON TABLE crm_deals IS 'CRM deals tracked through pipeline stages';

-- Add deferred FK on crm_leads.converted_deal_id now that crm_deals exists
ALTER TABLE crm_leads
    ADD CONSTRAINT fk_crm_leads_converted_deal
    FOREIGN KEY (converted_deal_id) REFERENCES crm_deals(id) ON DELETE SET NULL;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE crm_leads DROP CONSTRAINT IF EXISTS fk_crm_leads_converted_deal;
ALTER TABLE platform_contacts DROP CONSTRAINT IF EXISTS fk_pcon_company;

DROP TABLE IF EXISTS crm_deals;
DROP TABLE IF EXISTS crm_pipeline_stages;
DROP TABLE IF EXISTS crm_pipelines;
DROP TABLE IF EXISTS crm_leads;
DROP TABLE IF EXISTS platform_email_logs;
DROP TABLE IF EXISTS platform_activities;
DROP TABLE IF EXISTS platform_tasks;
DROP TABLE IF EXISTS platform_notes;
DROP TABLE IF EXISTS platform_contacts;
DROP TABLE IF EXISTS platform_companies;

-- +goose StatementEnd