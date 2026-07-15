-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00054_create_crm_templates
--
-- CRM Templates table to store email and note snippets for quick
-- insertion by sales reps.
-- ============================================================

CREATE TABLE crm_templates (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id    TEXT        NOT NULL UNIQUE DEFAULT ('tmpl_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id       UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    
    name         TEXT        NOT NULL,
    type         TEXT        NOT NULL CHECK (type IN ('email', 'note')),
    subject      TEXT,       -- Optional, generally used only for 'email' type
    body         TEXT        NOT NULL,
    
    created_by   UUID        NOT NULL REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_crm_templates_org_id ON crm_templates (org_id);
CREATE INDEX idx_crm_templates_type ON crm_templates (type);

COMMENT ON TABLE crm_templates IS 'CRM templates for standardizing emails and notes';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS crm_templates;
-- +goose StatementEnd
