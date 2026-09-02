-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00066_hrm_money_decimals
-- Add rounding policy to organizations and widen money columns.
-- ============================================================

-- Add rounding policy to organizations
ALTER TABLE organizations
    ADD COLUMN money_rounding_scale INTEGER NOT NULL DEFAULT 2,
    ADD COLUMN money_rounding_mode TEXT NOT NULL DEFAULT 'half_up'
        CHECK (money_rounding_mode IN ('half_up', 'half_even', 'down', 'up', 'ceiling', 'floor'));

COMMENT ON COLUMN organizations.money_rounding_scale IS 'Decimal places to round money values to';
COMMENT ON COLUMN organizations.money_rounding_mode IS 'Rounding mode for money values';

-- Widen hrm_payslips totals
ALTER TABLE hrm_payslips
    ALTER COLUMN gross_pay TYPE NUMERIC(18,4),
    ALTER COLUMN total_deductions TYPE NUMERIC(18,4),
    ALTER COLUMN net_pay TYPE NUMERIC(18,4),
    ALTER COLUMN basic_pay TYPE NUMERIC(18,4);

-- Widen hrm_payslip_runs totals
ALTER TABLE hrm_payslip_runs
    ALTER COLUMN total_gross_pay TYPE NUMERIC(18,4),
    ALTER COLUMN total_deductions TYPE NUMERIC(18,4),
    ALTER COLUMN total_net_pay TYPE NUMERIC(18,4);

-- Widen hrm_payslip_lines totals
ALTER TABLE hrm_payslip_lines
    ALTER COLUMN computed_amount TYPE NUMERIC(18,4);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE hrm_payslip_lines
    ALTER COLUMN computed_amount TYPE NUMERIC(15,2);

ALTER TABLE hrm_payslip_runs
    ALTER COLUMN total_gross_pay TYPE NUMERIC(18,2),
    ALTER COLUMN total_deductions TYPE NUMERIC(18,2),
    ALTER COLUMN total_net_pay TYPE NUMERIC(18,2);

ALTER TABLE hrm_payslips
    ALTER COLUMN gross_pay TYPE NUMERIC(15,2),
    ALTER COLUMN total_deductions TYPE NUMERIC(15,2),
    ALTER COLUMN net_pay TYPE NUMERIC(15,2),
    ALTER COLUMN basic_pay TYPE NUMERIC(15,2);

ALTER TABLE organizations
    DROP COLUMN IF EXISTS money_rounding_scale,
    DROP COLUMN IF EXISTS money_rounding_mode;

-- +goose StatementEnd
