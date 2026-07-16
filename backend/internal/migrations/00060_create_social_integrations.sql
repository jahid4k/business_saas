-- +goose Up
-- +goose StatementBegin

CREATE TABLE social_integrations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,
    page_id TEXT NOT NULL,
    form_id TEXT,
    access_token TEXT NOT NULL,
    field_mappings JSONB,
    is_active BOOLEAN NOT NULL DEFAULT true,
    connected_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_social_integrations_org_id ON social_integrations(org_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE social_integrations;

-- +goose StatementEnd
