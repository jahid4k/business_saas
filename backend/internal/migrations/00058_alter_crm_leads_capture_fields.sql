-- +goose Up
-- +goose StatementBegin

ALTER TABLE crm_leads
ADD COLUMN custom_fields JSONB DEFAULT '{}',
ADD COLUMN capture_source TEXT,
ADD COLUMN capture_metadata JSONB DEFAULT '{}';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE crm_leads
DROP COLUMN custom_fields,
DROP COLUMN capture_source,
DROP COLUMN capture_metadata;

-- +goose StatementEnd
