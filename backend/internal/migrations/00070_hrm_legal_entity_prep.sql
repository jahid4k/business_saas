-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00070_hrm_legal_entity_prep
-- Phase 0.4 prep migration. Minimal legal-entity scaffold ahead of
-- Phase 11 (multi-country/multi-currency) — zero business logic here,
-- just the column and a default row per org so nothing downstream
-- has to special-case NULLs later.
-- ============================================================

CREATE TABLE hrm_legal_entities (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    is_default  BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_legal_entities_org_id ON hrm_legal_entities(org_id);
CREATE UNIQUE INDEX idx_hrm_legal_entities_org_default ON hrm_legal_entities(org_id) WHERE is_default;

COMMENT ON TABLE hrm_legal_entities IS 'Minimal legal-entity scaffold for Phase 11 multi-country/multi-currency; no business logic yet.';

-- One default legal entity per existing org.
INSERT INTO hrm_legal_entities (org_id, name, is_default)
SELECT id, name, TRUE FROM organizations;

-- Add legal_entity_id to every HRM table that carries a direct org_id column,
-- backfilled to that org's default entity.
DO $$
DECLARE
    tbl TEXT;
    tables TEXT[] := ARRAY[
        'hrm_acknowledgements', 'hrm_announcements', 'hrm_approval_instances', 'hrm_approval_templates',
        'hrm_attendance_periods', 'hrm_attendance_records', 'hrm_awards', 'hrm_calendar_assignments',
        'hrm_calendar_events', 'hrm_complaints', 'hrm_departments', 'hrm_document_bulk_sends',
        'hrm_document_templates', 'hrm_employee_contracts', 'hrm_employee_documents', 'hrm_employee_milestones',
        'hrm_employee_salary_records', 'hrm_employee_statuses', 'hrm_employee_warnings', 'hrm_employees',
        'hrm_holiday_calendars', 'hrm_leave_requests', 'hrm_leave_types', 'hrm_payslip_lines',
        'hrm_payslip_runs', 'hrm_payslips', 'hrm_positions', 'hrm_promotions', 'hrm_resignations',
        'hrm_salary_components', 'hrm_salary_structures', 'hrm_shifts', 'hrm_terminations',
        'hrm_transfers', 'hrm_warning_escalation_rules', 'hrm_warning_types', 'hrm_work_schedule_assignments'
    ];
BEGIN
    FOREACH tbl IN ARRAY tables LOOP
        EXECUTE format('ALTER TABLE %I ADD COLUMN legal_entity_id UUID REFERENCES hrm_legal_entities(id)', tbl);
        EXECUTE format(
            'UPDATE %I t SET legal_entity_id = le.id FROM hrm_legal_entities le WHERE le.org_id = t.org_id AND le.is_default = TRUE',
            tbl
        );
    END LOOP;
END $$;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DO $$
DECLARE
    tbl TEXT;
    tables TEXT[] := ARRAY[
        'hrm_acknowledgements', 'hrm_announcements', 'hrm_approval_instances', 'hrm_approval_templates',
        'hrm_attendance_periods', 'hrm_attendance_records', 'hrm_awards', 'hrm_calendar_assignments',
        'hrm_calendar_events', 'hrm_complaints', 'hrm_departments', 'hrm_document_bulk_sends',
        'hrm_document_templates', 'hrm_employee_contracts', 'hrm_employee_documents', 'hrm_employee_milestones',
        'hrm_employee_salary_records', 'hrm_employee_statuses', 'hrm_employee_warnings', 'hrm_employees',
        'hrm_holiday_calendars', 'hrm_leave_requests', 'hrm_leave_types', 'hrm_payslip_lines',
        'hrm_payslip_runs', 'hrm_payslips', 'hrm_positions', 'hrm_promotions', 'hrm_resignations',
        'hrm_salary_components', 'hrm_salary_structures', 'hrm_shifts', 'hrm_terminations',
        'hrm_transfers', 'hrm_warning_escalation_rules', 'hrm_warning_types', 'hrm_work_schedule_assignments'
    ];
BEGIN
    FOREACH tbl IN ARRAY tables LOOP
        EXECUTE format('ALTER TABLE %I DROP COLUMN IF EXISTS legal_entity_id', tbl);
    END LOOP;
END $$;

DROP TABLE IF EXISTS hrm_legal_entities;

-- +goose StatementEnd
