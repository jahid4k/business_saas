-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00017_fix_audit_logs_business_id
--
-- audit_logs was created with org_id (migration 00011) but the
-- audit repository and Event struct use business_id throughout.
-- This migration adds business_id as an alias column so that
-- the repository INSERT works without renaming org_id (which
-- would break existing indexes and any direct SQL queries).
--
-- Both columns reference organizations(id):
--   org_id      — legacy name, kept for backward compatibility
--   business_id — name used by all application code going forward
-- ============================================================

ALTER TABLE audit_logs
    ADD COLUMN IF NOT EXISTS business_id UUID REFERENCES organizations(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_audit_logs_business_id ON audit_logs (business_id);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_audit_logs_business_id;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS business_id;

-- +goose StatementEnd