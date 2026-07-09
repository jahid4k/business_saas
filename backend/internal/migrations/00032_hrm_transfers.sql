-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00030_hrm_transfers
--
-- HRM employee transfer management (Group B2).
--
-- Status machine: draft → pending_approval → approved → applied
--                                   ↓              ↓
--                                rejected       cancelled
--
-- Transfer types:
--   department     → changes department_id on employee
--   location       → changes work location (text field — no location table yet)
--   reporting      → changes manager_id on employee
--   full           → department + location + reporting change together
--
-- Design notes:
--   • HR or manager initiates — employees cannot self-initiate transfers.
--     (ADR-0014: transfer is an employer-driven action, not self-service.)
--   • from_* fields are snapshots at record creation.
--   • When applied, employee.department_id and/or employee.manager_id updated.
--   • manager_id on hrm_employees is the manager's employee ID (self-referential),
--     not the user ID. The to_manager_employee_id references hrm_employees.
-- ============================================================

CREATE TABLE hrm_transfers (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id                TEXT        NOT NULL UNIQUE
                                             DEFAULT ('trf_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                   UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id              UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE RESTRICT,

    transfer_type            TEXT        NOT NULL DEFAULT 'department'
                                             CHECK (transfer_type IN ('department', 'location', 'reporting', 'full')),

    -- Snapshot of current state
    from_department_id       UUID        REFERENCES hrm_departments(id) ON DELETE SET NULL,
    from_manager_employee_id UUID        REFERENCES hrm_employees(id) ON DELETE SET NULL,
    from_location            TEXT,

    -- Target state
    to_department_id         UUID        REFERENCES hrm_departments(id) ON DELETE SET NULL,
    to_manager_employee_id   UUID        REFERENCES hrm_employees(id) ON DELETE SET NULL,
    to_location              TEXT,

    effective_date           DATE        NOT NULL,
    reason                   TEXT,
    notes                    TEXT,

    -- A2: approval instance
    approval_instance_id     UUID        REFERENCES hrm_approval_instances(id) ON DELETE SET NULL,

    -- A4: generated transfer letter
    document_id              UUID        REFERENCES hrm_employee_documents(id) ON DELETE SET NULL,

    status                   TEXT        NOT NULL DEFAULT 'draft'
                                             CHECK (status IN (
                                                 'draft', 'pending_approval', 'approved',
                                                 'rejected', 'cancelled', 'applied'
                                             )),

    applied_at               TIMESTAMPTZ,
    applied_by               UUID        REFERENCES users(id) ON DELETE SET NULL,

    created_by               UUID        NOT NULL REFERENCES users(id),
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_trf_org_id      ON hrm_transfers (org_id);
CREATE INDEX idx_hrm_trf_employee_id ON hrm_transfers (employee_id);
CREATE INDEX idx_hrm_trf_status      ON hrm_transfers (org_id, status);
CREATE INDEX idx_hrm_trf_effective   ON hrm_transfers (effective_date) WHERE status = 'approved';

COMMENT ON TABLE  hrm_transfers IS 'Employee transfer records: department, location, and/or reporting line change';
COMMENT ON COLUMN hrm_transfers.transfer_type            IS 'department | location | reporting | full';
COMMENT ON COLUMN hrm_transfers.to_manager_employee_id   IS 'References hrm_employees.id (self-FK), not users.id';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_transfers;

-- +goose StatementEnd
