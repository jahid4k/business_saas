-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00129_hrm_exchange_rates
--
-- Phase 11B-1: currency conversion gets a source. One table created, two
-- extended:
--
--   hrm_exchange_rates          — effective-dated FX rates
--   hrm_expense_lines           — EXTENDED with exchange_rate_date
--   hrm_exit_settlement_lines   — EXTENDED with the full conversion audit set
--
-- ⚠ THIS TABLE IS THE THING TWO SHIPPED SLICES DELIBERATELY REFUSED TO WORK
-- WITHOUT.
--
-- internal/hrm/exits/settlement.go reports a foreign-currency travel advance
-- at ZERO with "NOT RECOVERED: no exchange rate", because "converting a
-- foreign-currency advance would mean inventing a rate and mis-charging a
-- departing person real money". internal/hrm/expenses/money.go freezes
-- whatever rate the claimant typed, because there was nowhere to look one up.
-- Both were correct refusals. This migration gives them something to read.
--
-- ⚠ CURRENCY RULE: NEVER STORE CONVERTED-ONLY.
--
-- Every converted figure in this system keeps all five of:
--
--     original_amount + original_currency + rate + rate_date + converted_amount
--
-- A stored conversion carrying only the converted number cannot be audited,
-- cannot be recomputed when a rate is corrected, and cannot be explained to
-- the person whose settlement it reduced. hrm_expense_lines already had four
-- of the five (amount, currency, exchange_rate, base_amount) — 8B's instinct
-- was right — and is one column short. hrm_exit_settlement_lines had none and
-- gains the set.
--
-- ⚠ rate IS NUMERIC(18,8), NOT (15,2). A rate is not money. Rounding
-- 0.00000123 to 0.00 before multiplying turns a real balance into nothing,
-- and rounding 1.0857 to 1.09 misprices every line it touches. Only the
-- RESULT of a conversion is money and rounds to 2 places.
--
-- ⚠ Every new column is NULLABLE. Existing expense lines and settlement
-- lines record no conversion, and writing one in now would be inventing the
-- rate this table exists to stop people inventing.
-- ============================================================

CREATE TABLE hrm_exchange_rates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT NOT NULL UNIQUE
                    DEFAULT 'fxrate_' || replace(gen_random_uuid()::text, '-', ''),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    from_currency   CHAR(3) NOT NULL,
    to_currency     CHAR(3) NOT NULL,
    rate            NUMERIC(18,8) NOT NULL,
    rate_date       DATE NOT NULL,

    source          TEXT NOT NULL DEFAULT 'manual',
    note            TEXT,

    created_by      UUID NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- No updated_at: a rate for a given day is a historical fact. Correcting
    -- one is inserting the corrected value, not editing the record of what
    -- was believed at the time.
    CONSTRAINT chk_hrm_fx_from     CHECK (from_currency ~ '^[A-Z]{3}$'),
    CONSTRAINT chk_hrm_fx_to       CHECK (to_currency ~ '^[A-Z]{3}$'),
    CONSTRAINT chk_hrm_fx_positive CHECK (rate > 0),
    -- A self-rate is either 1 (useless) or wrong (dangerous). Refusing it
    -- means no lookup can ever return a USD→USD rate of 0.98.
    CONSTRAINT chk_hrm_fx_distinct CHECK (from_currency <> to_currency),
    CONSTRAINT chk_hrm_fx_source   CHECK (source IN ('manual', 'import'))
);

-- One rate per pair per day. Two rates for USD→EUR on the same date is two
-- answers to a question that has one, and the effective-dated lookup would
-- pick between them arbitrarily.
CREATE UNIQUE INDEX uq_hrm_fx_pair_date
    ON hrm_exchange_rates (org_id, from_currency, to_currency, rate_date);
-- Supports the MAX(rate_date) <= asOf lookup (the SlabsAsOf shape).
CREATE INDEX idx_hrm_fx_lookup
    ON hrm_exchange_rates (org_id, from_currency, to_currency, rate_date DESC);

-- ── hrm_expense_lines: the missing fifth field ───────────────────────────────
--
-- 8B stored amount + currency + exchange_rate + base_amount. That is four of
-- the five, and the missing one is WHEN the rate was true — without it a
-- stored rate cannot be checked against the table it supposedly came from.
ALTER TABLE hrm_expense_lines
    ADD COLUMN exchange_rate_date DATE;

-- ── hrm_exit_settlement_lines: the whole set ─────────────────────────────────
--
-- ⚠ amount AND currency KEEP THEIR EXISTING MEANING — the CONVERTED figure in
-- the run's currency. Every existing reader (payslip assembly through
-- payslip_line_id, the settlement trail, the F&F totals) goes on working
-- untouched, and the original is recorded ALONGSIDE rather than replacing it.
-- Redefining amount to mean the original would have silently changed what
-- every already-settled row means.
ALTER TABLE hrm_exit_settlement_lines
    ADD COLUMN original_amount    NUMERIC(15,2),
    ADD COLUMN original_currency  CHAR(3),
    ADD COLUMN exchange_rate      NUMERIC(18,8),
    ADD COLUMN exchange_rate_date DATE;

-- All-or-nothing: a line either records a conversion completely or not at
-- all. A half-recorded conversion is the converted-only case wearing four
-- columns.
ALTER TABLE hrm_exit_settlement_lines
    ADD CONSTRAINT chk_hrm_esl_conversion_complete CHECK (
        (original_amount IS NULL AND original_currency IS NULL
             AND exchange_rate IS NULL AND exchange_rate_date IS NULL)
        OR
        (original_amount IS NOT NULL AND original_currency IS NOT NULL
             AND exchange_rate IS NOT NULL AND exchange_rate_date IS NOT NULL)
    ),
    ADD CONSTRAINT chk_hrm_esl_rate_positive CHECK (
        exchange_rate IS NULL OR exchange_rate > 0
    ),
    ADD CONSTRAINT chk_hrm_esl_orig_currency CHECK (
        original_currency IS NULL OR original_currency ~ '^[A-Z]{3}$'
    );

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- ⚠ The two ALTERed tables pre-date this migration with rows in them, so they
-- are RESTORED to their exact prior shape rather than dropped. Constraints
-- come off before their columns.
ALTER TABLE hrm_exit_settlement_lines
    DROP CONSTRAINT IF EXISTS chk_hrm_esl_conversion_complete,
    DROP CONSTRAINT IF EXISTS chk_hrm_esl_rate_positive,
    DROP CONSTRAINT IF EXISTS chk_hrm_esl_orig_currency;

ALTER TABLE hrm_exit_settlement_lines
    DROP COLUMN IF EXISTS original_amount,
    DROP COLUMN IF EXISTS original_currency,
    DROP COLUMN IF EXISTS exchange_rate,
    DROP COLUMN IF EXISTS exchange_rate_date;

ALTER TABLE hrm_expense_lines
    DROP COLUMN IF EXISTS exchange_rate_date;

DROP TABLE IF EXISTS hrm_exchange_rates;

-- +goose StatementEnd
