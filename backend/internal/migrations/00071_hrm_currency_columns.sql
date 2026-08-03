-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00071_hrm_currency_columns
-- Phase 0.4 prep migration. Adds currency alongside money columns that
-- don't have one yet, and standardizes the ad-hoc TEXT currency columns
-- already in use to CHAR(3) (ISO 4217 codes are always exactly 3 letters,
-- so CHAR(3) never blank-pads here).
-- ============================================================

ALTER TABLE hrm_employee_salary_records ADD COLUMN currency CHAR(3) NOT NULL DEFAULT 'USD';
UPDATE hrm_employee_salary_records t SET currency = o.currency FROM organizations o WHERE o.id = t.org_id;

ALTER TABLE hrm_promotions ADD COLUMN currency CHAR(3) NOT NULL DEFAULT 'USD';
UPDATE hrm_promotions t SET currency = o.currency FROM organizations o WHERE o.id = t.org_id;

ALTER TABLE hrm_salary_components ADD COLUMN currency CHAR(3) NOT NULL DEFAULT 'USD';
UPDATE hrm_salary_components t SET currency = o.currency FROM organizations o WHERE o.id = t.org_id;

ALTER TABLE hrm_payslip_lines ADD COLUMN currency CHAR(3) NOT NULL DEFAULT 'USD';
UPDATE hrm_payslip_lines t SET currency = o.currency FROM organizations o WHERE o.id = t.org_id;

-- hrm_salary_structure_components has no direct org_id; scoped via structure_id.
ALTER TABLE hrm_salary_structure_components ADD COLUMN currency CHAR(3) NOT NULL DEFAULT 'USD';
UPDATE hrm_salary_structure_components t
SET currency = o.currency
FROM hrm_salary_structures s JOIN organizations o ON o.id = s.org_id
WHERE s.id = t.structure_id;

-- Standardize existing ad-hoc TEXT currency columns to CHAR(3).
ALTER TABLE organizations ALTER COLUMN currency TYPE CHAR(3);
ALTER TABLE hrm_awards ALTER COLUMN currency TYPE CHAR(3);
ALTER TABLE hrm_terminations ALTER COLUMN severance_currency TYPE CHAR(3);
ALTER TABLE hrm_payslips ALTER COLUMN currency TYPE CHAR(3);
ALTER TABLE hrm_payslip_runs ALTER COLUMN currency TYPE CHAR(3);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE organizations ALTER COLUMN currency TYPE TEXT;
ALTER TABLE hrm_awards ALTER COLUMN currency TYPE TEXT;
ALTER TABLE hrm_terminations ALTER COLUMN severance_currency TYPE TEXT;
ALTER TABLE hrm_payslips ALTER COLUMN currency TYPE TEXT;
ALTER TABLE hrm_payslip_runs ALTER COLUMN currency TYPE TEXT;

ALTER TABLE hrm_salary_structure_components DROP COLUMN currency;
ALTER TABLE hrm_payslip_lines DROP COLUMN currency;
ALTER TABLE hrm_salary_components DROP COLUMN currency;
ALTER TABLE hrm_promotions DROP COLUMN currency;
ALTER TABLE hrm_employee_salary_records DROP COLUMN currency;

-- +goose StatementEnd
