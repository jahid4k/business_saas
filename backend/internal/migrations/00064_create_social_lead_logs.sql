-- +goose Up
-- +goose StatementBegin

CREATE TABLE social_lead_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,
    page_id TEXT NOT NULL,
    raw_payload JSONB NOT NULL,
    processed BOOLEAN NOT NULL DEFAULT false,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_social_lead_logs_org_id ON social_lead_logs(org_id);
CREATE INDEX idx_social_lead_logs_page_id ON social_lead_logs(page_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE social_lead_logs;

-- +goose StatementEnd
