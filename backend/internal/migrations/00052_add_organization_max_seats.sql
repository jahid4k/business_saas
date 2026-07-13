
-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00052_add_organization_max_seats
-- Simple, manually-set member-seat cap per organization.
-- NULL = unlimited. Deliberately NOT tied to organization_usage /
-- subscriptions — those are billing-period based and there's no
-- live billing system yet. This is a standalone MVP guard rail.
-- ============================================================

ALTER TABLE organizations ADD COLUMN max_seats INT;

COMMENT ON COLUMN organizations.max_seats IS 'Manually-set cap on active members. NULL = unlimited.';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE organizations DROP COLUMN IF EXISTS max_seats;

-- +goose StatementEnd