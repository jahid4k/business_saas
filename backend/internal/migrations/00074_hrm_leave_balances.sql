-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00074_hrm_leave_balances
--
-- HRM Phase 2 (Leave Engine Upgrade): adds balance tracking on top of the
-- existing hrm_leave_types / hrm_leave_requests tables. Purely additive —
-- a leave type with no hrm_leave_policies row continues behaving exactly
-- as it does today, zero balance enforcement. Balance tracking activates
-- per (org, leave_type) the moment a policy row is created for it.
--
-- Three tables:
--   hrm_leave_policies      — per (org, leave_type) accrual/carry-forward/
--                             encashment config. One active row per pair.
--   hrm_leave_transactions  — append-only ledger, the source of truth.
--                             Signed `days` column: credits positive,
--                             debits negative. Never UPDATEd or DELETEd.
--   hrm_leave_balances      — immutable monthly snapshot rows, modeled on
--                             hrm_employee_salary_records' effective-dated
--                             pattern (single as_of_date, no effective_to,
--                             "current" = ORDER BY ... DESC LIMIT 1).
-- ============================================================

-- ------------------------------------------------------------
-- hrm_leave_policies
-- ------------------------------------------------------------
CREATE TABLE hrm_leave_policies (
    id                     UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id              TEXT        NOT NULL UNIQUE
                                           DEFAULT ('lvp_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                 UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    leave_type_id          UUID        NOT NULL REFERENCES hrm_leave_types(id) ON DELETE RESTRICT,

    accrual_method         TEXT        NOT NULL
                                           CHECK (accrual_method IN ('monthly', 'annual', 'on_joining')),
    accrual_rate           NUMERIC(6,2) NOT NULL DEFAULT 0 CHECK (accrual_rate >= 0),
    -- meaning depends on accrual_method: monthly="days granted per month",
    -- annual="days granted per year", on_joining="one-time total at hire"

    carry_forward_enabled  BOOLEAN     NOT NULL DEFAULT FALSE,
    carry_forward_cap      NUMERIC(6,2) CHECK (carry_forward_cap IS NULL OR carry_forward_cap >= 0),
    -- NULL cap while enabled = uncapped carry-forward

    encashable             BOOLEAN     NOT NULL DEFAULT FALSE,
    encashment_rate_basis  TEXT        CHECK (encashment_rate_basis IS NULL OR encashment_rate_basis IN ('basic_pay', 'gross_pay', 'fixed')),
    -- stored only — Phase 2 never evaluates this; Phase 9 (F&F) reads it

    is_active              BOOLEAN     NOT NULL DEFAULT TRUE,
    created_by             UUID        NOT NULL REFERENCES users(id),
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_lvp_carry_cap CHECK (carry_forward_enabled = TRUE OR carry_forward_cap IS NULL),
    CONSTRAINT chk_hrm_lvp_encash_basis CHECK (encashable = TRUE OR encashment_rate_basis IS NULL)
);

CREATE INDEX idx_hrm_lvp_org_id ON hrm_leave_policies (org_id);
CREATE UNIQUE INDEX idx_hrm_lvp_org_leave_type ON hrm_leave_policies (org_id, leave_type_id) WHERE is_active = TRUE;

COMMENT ON TABLE hrm_leave_policies IS 'Per (org, leave_type) accrual/carry-forward/encashment configuration — one active policy per pair';
COMMENT ON COLUMN hrm_leave_policies.accrual_rate IS 'Meaning depends on accrual_method: days/month, days/year, or one-time total at hire';
COMMENT ON COLUMN hrm_leave_policies.carry_forward_cap IS 'NULL while carry_forward_enabled=TRUE means uncapped';
COMMENT ON COLUMN hrm_leave_policies.encashment_rate_basis IS 'Config only, stored for Phase 9 (F&F) to read — never evaluated in this phase';

-- ------------------------------------------------------------
-- hrm_leave_transactions (append-only ledger — source of truth)
-- ------------------------------------------------------------
CREATE TABLE hrm_leave_transactions (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id         TEXT         NOT NULL UNIQUE
                                       DEFAULT ('lvx_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id            UUID         NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id       UUID         NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,
    leave_type_id     UUID         NOT NULL REFERENCES hrm_leave_types(id) ON DELETE RESTRICT,
    policy_id         UUID         NOT NULL REFERENCES hrm_leave_policies(id) ON DELETE RESTRICT,

    transaction_type  TEXT         NOT NULL
                                       CHECK (transaction_type IN
                                           ('accrual', 'usage', 'usage_reversal', 'encashment',
                                            'carry_forward', 'forfeiture', 'adjustment')),
    days              NUMERIC(6,2) NOT NULL CHECK (days <> 0),
    -- SIGNED: credits (accrual, usage_reversal, carry_forward, adjustment-up)
    -- positive; debits (usage, encashment, forfeiture, adjustment-down) negative
    transaction_date  DATE         NOT NULL,

    leave_request_id  UUID         REFERENCES hrm_leave_requests(id) ON DELETE SET NULL,
    note              TEXT,

    created_by        UUID         NOT NULL REFERENCES users(id),
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
    -- append-only: no updated_at, no UPDATE/DELETE repository methods
);

CREATE INDEX idx_hrm_lvx_org_id ON hrm_leave_transactions (org_id);
CREATE INDEX idx_hrm_lvx_emp_type_date ON hrm_leave_transactions (employee_id, leave_type_id, transaction_date);
CREATE INDEX idx_hrm_lvx_leave_request ON hrm_leave_transactions (leave_request_id) WHERE leave_request_id IS NOT NULL;

-- Idempotency backstop for the accrual and carry-forward jobs — the
-- scheduler's Redis lock has no renewal, so a run exceeding its TTL could
-- double-execute.
CREATE UNIQUE INDEX uq_hrm_lvx_accrual_period ON hrm_leave_transactions (employee_id, leave_type_id, transaction_date)
    WHERE transaction_type = 'accrual';
CREATE UNIQUE INDEX uq_hrm_lvx_forfeiture_period ON hrm_leave_transactions (employee_id, leave_type_id, transaction_date)
    WHERE transaction_type = 'forfeiture';

-- Prevents double-posting usage/usage_reversal for the same request — defense
-- in depth alongside the service-layer status guards.
CREATE UNIQUE INDEX uq_hrm_lvx_request_type ON hrm_leave_transactions (leave_request_id, transaction_type)
    WHERE leave_request_id IS NOT NULL;

COMMENT ON TABLE hrm_leave_transactions IS 'Append-only leave balance ledger — the source of truth. Balance is always derived from this table, never stored as a mutable current value.';
COMMENT ON COLUMN hrm_leave_transactions.days IS 'Signed: positive = credit, negative = debit';

-- ------------------------------------------------------------
-- hrm_leave_balances (immutable monthly snapshot rows)
-- ------------------------------------------------------------
CREATE TABLE hrm_leave_balances (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id         TEXT         NOT NULL UNIQUE
                                       DEFAULT ('lvb_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id            UUID         NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id       UUID         NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,
    leave_type_id     UUID         NOT NULL REFERENCES hrm_leave_types(id) ON DELETE RESTRICT,
    policy_id         UUID         NOT NULL REFERENCES hrm_leave_policies(id) ON DELETE RESTRICT,

    period_year       INTEGER      NOT NULL,
    period_month      INTEGER      NOT NULL CHECK (period_month BETWEEN 1 AND 12),
    as_of_date        DATE         NOT NULL,  -- always the 1st of period_month

    opening_balance   NUMERIC(6,2) NOT NULL,
    accrued           NUMERIC(6,2) NOT NULL DEFAULT 0,
    taken             NUMERIC(6,2) NOT NULL DEFAULT 0,
    encashed          NUMERIC(6,2) NOT NULL DEFAULT 0,
    carried_forward   NUMERIC(6,2) NOT NULL DEFAULT 0,
    adjusted          NUMERIC(6,2) NOT NULL DEFAULT 0,
    closing_balance   NUMERIC(6,2) NOT NULL,

    created_by        UUID         NOT NULL REFERENCES users(id),
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
    -- no updated_at: rows are never mutated after insert, matching the
    -- hrm_employee_salary_records effective-dated pattern
);

CREATE UNIQUE INDEX uq_hrm_lvb_period ON hrm_leave_balances (employee_id, leave_type_id, period_year, period_month);
CREATE INDEX idx_hrm_lvb_emp_type_period ON hrm_leave_balances (employee_id, leave_type_id, period_year DESC, period_month DESC);

COMMENT ON TABLE hrm_leave_balances IS 'Immutable monthly balance snapshots, one row per (employee, leave_type, period). "Current" balance = latest row''s closing_balance + ledger transactions since its as_of_date.';
COMMENT ON COLUMN hrm_leave_balances.adjusted IS 'Sum of manual adjustment transactions this period — kept as its own line so opening+accrued-taken-encashed+carried_forward+adjusted reconciles against closing_balance';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_leave_balances;
DROP TABLE IF EXISTS hrm_leave_transactions;
DROP TABLE IF EXISTS hrm_leave_policies;

-- +goose StatementEnd
