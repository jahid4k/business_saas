-- +goose Up
-- +goose StatementBegin

CREATE TABLE org_inbound_emails (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    address TEXT NOT NULL UNIQUE,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_org_inbound_emails_org_id ON org_inbound_emails(org_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE org_inbound_emails;

-- +goose StatementEnd
