-- +goose Up
-- +goose StatementBegin

CREATE TABLE crm_settings (
    org_id UUID PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    
    lead_routing_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    round_robin_assignees JSONB NOT NULL DEFAULT '[]'::JSONB,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE crm_settings IS 'CRM specific settings for an organization';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE crm_settings;
-- +goose StatementEnd
