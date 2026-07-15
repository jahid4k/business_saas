-- +goose Up
-- +goose StatementBegin

CREATE TABLE inbound_email_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    to_address TEXT NOT NULL,
    from_address TEXT NOT NULL,
    subject TEXT,
    raw_payload JSONB NOT NULL,
    processed BOOLEAN NOT NULL DEFAULT false,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_inbound_email_logs_org_id ON inbound_email_logs(org_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE inbound_email_logs;

-- +goose StatementEnd
