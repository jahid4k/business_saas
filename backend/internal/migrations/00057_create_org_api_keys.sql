-- +goose Up
-- +goose StatementBegin

CREATE TABLE org_api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    key_prefix TEXT NOT NULL,
    key_hash TEXT NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    allowed_origins TEXT[] NOT NULL DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_org_api_keys_org_id ON org_api_keys(org_id);
CREATE INDEX idx_org_api_keys_key_hash ON org_api_keys(key_hash);
CREATE INDEX idx_org_api_keys_prefix ON org_api_keys(key_prefix);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE org_api_keys;

-- +goose StatementEnd
