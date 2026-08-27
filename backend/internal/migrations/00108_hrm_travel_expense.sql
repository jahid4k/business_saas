-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00108_hrm_travel_expense
--
-- Phase 8B: travel and expense. Nine tables:
--
--   hrm_travel_requests           — a trip, approval-gated
--   hrm_travel_itinerary_items    — legs within a trip
--   hrm_travel_advances           — money paid BEFORE the trip
--   hrm_expense_claims            — a claim, approval-gated
--   hrm_expense_lines             — the claim's line items; APPROVAL LIVES HERE
--   hrm_expense_policies          — per-category caps, effective-dated
--   hrm_per_diem_rates            — per-destination daily allowance, effective-dated
--   hrm_mileage_rates             — per-unit travel rate, effective-dated
--   hrm_expense_policy_violations — recorded warnings, never blocks
--
-- Design notes:
--
--   • APPROVAL IS LINE-LEVEL, NOT CLAIM-LEVEL. The build plan is explicit:
--     "amount vs approved_amount per line, not claim-level". An approver
--     reduces individual lines — "I'll cover the flight but not the minibar"
--     — so hrm_expense_lines carries both amount and approved_amount, and
--     hrm_expense_claims carries NEITHER total. The claim's totals are
--     SUM(lines.*) computed at read time (the 00076 rule): a stored
--     claim.total_approved would silently disagree with its own lines the
--     first time one was adjusted, which is the exact failure r25 found in
--     hrm_payslip_runs (TotalEmployees counting rows the money totals did not).
--
--   • MULTI-CURRENCY WITHOUT AN FX SUBSYSTEM. The build plan says currency
--     columns are mandatory here "regardless of Phase 11", and there is no
--     exchange-rate table anywhere in this codebase today — building one is
--     Phase 11 scope. So each LINE snapshots the exchange_rate used and the
--     resulting base_amount at claim time. A later rate change cannot rewrite
--     a settled claim, because nothing re-derives it. Identical discipline to
--     7B's calculation_snapshot and 7D's employee_cost_snapshot: the decision
--     is frozen, the catalog stays mutable.
--
--     exchange_rate DEFAULTs to 1 and base_amount then equals amount, so a
--     single-currency org never sees any of this.
--
--   • POLICY VIOLATIONS ARE WARNINGS, NEVER HARD BLOCKS. Again the build
--     plan's own words. A violation is a ROW in hrm_expense_policy_violations,
--     not a boolean on the line and not a rejected insert: the claim submits
--     successfully and the approver sees what breached what. A boolean could
--     not say WHICH policy or by how much, and a hard block would make an
--     over-cap taxi fare unclaimable rather than reviewable.
--
--   • RATES ARE EFFECTIVE-DATED, and read with the 7D SlabsAsOf shape
--     (MAX(effective_date) <= asOf). A per-diem raised next month must not
--     change what last month's trip was owed.
--
--   • ADVANCES: settled_amount is mutable and increases toward amount, the
--     hrm_loan_schedules.recovered_amount shape (00100). All THREE settlement
--     outcomes are real and each is tested: advance > claim (employee owes
--     the balance back), advance < claim (org owes the difference), and
--     advance == claim (clean). None of the three is an error state.
--
--   • ocr_raw is a nullable JSONB column and nothing writes it yet — manual
--     entry only, vendor later, exactly as scoped. It is the one place a
--     "build it before there is a consumer" exception is explicitly
--     sanctioned by the build plan, and it costs one nullable column.
--
--   • Claim payout does NOT happen here. An approved claim creates a
--     hrm_reimbursements row (00100), which already flows into payroll via
--     7C's payslips.ReimbursementSource. "Claim lifecycle here, payout via
--     payroll in compensation" — so this migration adds no payroll coupling
--     and internal/hrm/payslips is untouched by 8B.
--
-- What must NEVER be added (the 00076 CHECK x ON DELETE SET NULL trap):
-- Postgres re-evaluates CHECKs on UPDATE and ON DELETE SET NULL *is* an
-- UPDATE. hrm_expense_claims.travel_request_id, .reimbursement_id and
-- hrm_travel_advances.travel_request_id are all ON DELETE SET NULL, so:
--   • CHECK (travel_request_id IS NOT NULL OR expense_type <> 'travel')
--     would make DELETE FROM hrm_travel_requests fail 23514.
--   • CHECK (status <> 'paid' OR reimbursement_id IS NOT NULL) would make
--     DELETE FROM hrm_reimbursements fail the same way.
-- The service validates both pairings instead.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_travel_requests
-- ------------------------------------------------------------
CREATE TABLE hrm_travel_requests (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id             TEXT        NOT NULL UNIQUE
                                          DEFAULT ('trv_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id           UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    purpose               TEXT        NOT NULL,
    destination           TEXT        NOT NULL,
    -- Country code drives per-diem lookup. Free-text destination is the
    -- human label; the code is what the rate table matches on.
    destination_country   CHAR(2),
    start_date            DATE        NOT NULL,
    end_date              DATE        NOT NULL,

    status                TEXT        NOT NULL DEFAULT 'draft'
                                          CHECK (status IN (
                                              'draft', 'pending_approval', 'approved',
                                              'rejected', 'completed', 'cancelled'
                                          )),
    approval_instance_id  UUID        REFERENCES hrm_approval_instances(id) ON DELETE SET NULL,
    currency              CHAR(3)     NOT NULL DEFAULT 'USD',

    created_by            UUID        NOT NULL REFERENCES users(id),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_trv_dates CHECK (end_date >= start_date)
);

CREATE INDEX idx_hrm_trv_org_id   ON hrm_travel_requests (org_id, status);
CREATE INDEX idx_hrm_trv_employee ON hrm_travel_requests (employee_id);

-- ------------------------------------------------------------
-- hrm_travel_itinerary_items
-- ------------------------------------------------------------
CREATE TABLE hrm_travel_itinerary_items (
    id                 UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id          TEXT          NOT NULL UNIQUE
                                         DEFAULT ('itin_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    travel_request_id  UUID          NOT NULL REFERENCES hrm_travel_requests(id) ON DELETE CASCADE,

    item_type          TEXT          NOT NULL
                                         CHECK (item_type IN ('flight', 'train', 'hotel', 'car_rental', 'other')),
    description        TEXT,
    from_location      TEXT,
    to_location        TEXT,
    starts_at          TIMESTAMPTZ,
    ends_at            TIMESTAMPTZ,
    booking_reference  TEXT,
    estimated_cost     NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (estimated_cost >= 0),
    currency           CHAR(3)       NOT NULL DEFAULT 'USD',
    display_order      INTEGER       NOT NULL DEFAULT 0,

    created_at         TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_itin_times CHECK (ends_at IS NULL OR starts_at IS NULL OR ends_at >= starts_at)
);

CREATE INDEX idx_hrm_itin_request ON hrm_travel_itinerary_items (travel_request_id, display_order);

-- ------------------------------------------------------------
-- hrm_travel_advances
-- ------------------------------------------------------------
CREATE TABLE hrm_travel_advances (
    id                 UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id          TEXT          NOT NULL UNIQUE
                                         DEFAULT ('adv_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id             UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id        UUID          NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,
    travel_request_id  UUID          REFERENCES hrm_travel_requests(id) ON DELETE SET NULL,

    amount             NUMERIC(15,2) NOT NULL CHECK (amount > 0),
    currency           CHAR(3)       NOT NULL DEFAULT 'USD',
    -- The only mutable money here. Increases toward amount as claims settle
    -- against it — the hrm_loan_schedules.recovered_amount shape (00100).
    settled_amount     NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (settled_amount >= 0),

    status             TEXT          NOT NULL DEFAULT 'pending'
                                         CHECK (status IN ('pending', 'disbursed', 'settled', 'cancelled')),
    disbursed_at       TIMESTAMPTZ,
    disbursed_by       UUID          REFERENCES users(id) ON DELETE SET NULL,

    created_by         UUID          NOT NULL REFERENCES users(id),
    created_at         TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_adv_settled CHECK (settled_amount <= amount)
);

CREATE INDEX idx_hrm_adv_org_id   ON hrm_travel_advances (org_id, status);
CREATE INDEX idx_hrm_adv_employee ON hrm_travel_advances (employee_id);
CREATE INDEX idx_hrm_adv_request  ON hrm_travel_advances (travel_request_id) WHERE travel_request_id IS NOT NULL;

COMMENT ON COLUMN hrm_travel_advances.settled_amount IS 'Increases toward amount as claims settle against it; amount itself is frozen. Over-settlement (advance > claim) leaves a balance the employee owes back — see 00108 header.';

-- ------------------------------------------------------------
-- hrm_expense_claims
--
-- NOTE the absence of total_amount / total_approved_amount. Both are
-- SUM(lines) at read time — see migration header.
-- ------------------------------------------------------------
CREATE TABLE hrm_expense_claims (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id             TEXT        NOT NULL UNIQUE
                                          DEFAULT ('exp_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id           UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,
    travel_request_id     UUID        REFERENCES hrm_travel_requests(id) ON DELETE SET NULL,
    advance_id            UUID        REFERENCES hrm_travel_advances(id) ON DELETE SET NULL,

    title                 TEXT        NOT NULL,
    description           TEXT,
    -- The org's own currency. Every line's base_amount is expressed in this,
    -- which is what makes the claim's totals summable across mixed-currency
    -- lines at all.
    base_currency         CHAR(3)     NOT NULL DEFAULT 'USD',

    status                TEXT        NOT NULL DEFAULT 'draft'
                                          CHECK (status IN (
                                              'draft', 'pending_approval', 'approved',
                                              'partially_approved', 'rejected', 'paid', 'cancelled'
                                          )),
    approval_instance_id  UUID        REFERENCES hrm_approval_instances(id) ON DELETE SET NULL,
    -- Set once an approved claim has produced a payout record. The
    -- reimbursement is what payroll actually pays (7C); this is the link
    -- back, nullable because a claim is not born paid.
    reimbursement_id      UUID        REFERENCES hrm_reimbursements(id) ON DELETE SET NULL,

    submitted_at          TIMESTAMPTZ,
    decided_at            TIMESTAMPTZ,

    created_by            UUID        NOT NULL REFERENCES users(id),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_exp_org_id   ON hrm_expense_claims (org_id, status);
CREATE INDEX idx_hrm_exp_employee ON hrm_expense_claims (employee_id);
CREATE INDEX idx_hrm_exp_travel   ON hrm_expense_claims (travel_request_id) WHERE travel_request_id IS NOT NULL;

COMMENT ON TABLE hrm_expense_claims IS 'Claim header. There is deliberately NO total_amount or total_approved_amount column — both are SUM over hrm_expense_lines at read time, because approval is per-line and a stored total would drift the first time one line was adjusted.';

-- ------------------------------------------------------------
-- hrm_expense_lines — where approval actually happens
-- ------------------------------------------------------------
CREATE TABLE hrm_expense_lines (
    id              UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT          NOT NULL UNIQUE
                                      DEFAULT ('expl_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    claim_id        UUID          NOT NULL REFERENCES hrm_expense_claims(id) ON DELETE CASCADE,

    category        TEXT          NOT NULL
                                      CHECK (category IN (
                                          'airfare', 'lodging', 'meals', 'ground_transport',
                                          'mileage', 'per_diem', 'supplies', 'other'
                                      )),
    description     TEXT,
    expense_date    DATE          NOT NULL,

    -- What the employee actually spent, in the currency they spent it.
    amount          NUMERIC(15,2) NOT NULL CHECK (amount >= 0),
    currency        CHAR(3)       NOT NULL DEFAULT 'USD',

    -- FROZEN AT CLAIM TIME. There is no FX rate table in this codebase; a
    -- later rate change must not rewrite a settled claim. See migration header.
    exchange_rate   NUMERIC(18,8) NOT NULL DEFAULT 1 CHECK (exchange_rate > 0),
    base_amount     NUMERIC(15,2) NOT NULL CHECK (base_amount >= 0),

    -- THE line-level approval field. NULL means "not yet decided"; 0 means
    -- "decided, and nothing is payable" — a real and different outcome, so
    -- this must stay nullable rather than defaulting to 0.
    approved_amount NUMERIC(15,2) CHECK (approved_amount IS NULL OR approved_amount >= 0),

    -- Manual entry today; a vendor may populate this later, as scoped.
    receipt_url     TEXT,
    ocr_raw         JSONB,

    -- Mileage lines only: distance x rate. Both nullable because every other
    -- category leaves them empty.
    mileage_distance NUMERIC(12,2) CHECK (mileage_distance IS NULL OR mileage_distance >= 0),
    mileage_rate_id  UUID,

    display_order   INTEGER       NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_expl_approved_le_amount
        CHECK (approved_amount IS NULL OR approved_amount <= base_amount)
);

CREATE INDEX idx_hrm_expl_claim ON hrm_expense_lines (claim_id, display_order);

COMMENT ON COLUMN hrm_expense_lines.approved_amount IS 'NULL = undecided, 0 = decided and nothing payable. Two different states, which is why this is nullable rather than DEFAULT 0.';
COMMENT ON COLUMN hrm_expense_lines.exchange_rate  IS 'Snapshotted at claim time — there is no FX rate table, and a later rate change must never rewrite a settled claim';

-- ------------------------------------------------------------
-- hrm_expense_policies — effective-dated per-category caps
-- ------------------------------------------------------------
CREATE TABLE hrm_expense_policies (
    id             UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id      TEXT          NOT NULL UNIQUE
                                     DEFAULT ('exppol_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id         UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    category       TEXT          NOT NULL
                                     CHECK (category IN (
                                         'airfare', 'lodging', 'meals', 'ground_transport',
                                         'mileage', 'per_diem', 'supplies', 'other'
                                     )),
    max_amount     NUMERIC(15,2) NOT NULL CHECK (max_amount >= 0),
    currency       CHAR(3)       NOT NULL DEFAULT 'USD',
    effective_date DATE          NOT NULL,

    created_by     UUID          NOT NULL REFERENCES users(id),
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_exppol_lookup ON hrm_expense_policies (org_id, category, effective_date DESC);

-- ------------------------------------------------------------
-- hrm_per_diem_rates / hrm_mileage_rates — effective-dated
-- ------------------------------------------------------------
CREATE TABLE hrm_per_diem_rates (
    id             UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id      TEXT          NOT NULL UNIQUE
                                     DEFAULT ('pdm_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id         UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- NULL country_code is the org-wide fallback rate. A specific country
    -- row wins over it; resolution order lives in the service.
    country_code   CHAR(2),
    daily_amount   NUMERIC(15,2) NOT NULL CHECK (daily_amount >= 0),
    currency       CHAR(3)       NOT NULL DEFAULT 'USD',
    effective_date DATE          NOT NULL,

    created_by     UUID          NOT NULL REFERENCES users(id),
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_pdm_lookup ON hrm_per_diem_rates (org_id, country_code, effective_date DESC);

CREATE TABLE hrm_mileage_rates (
    id             UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id      TEXT          NOT NULL UNIQUE
                                     DEFAULT ('mil_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id         UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    rate_per_unit  NUMERIC(15,4) NOT NULL CHECK (rate_per_unit >= 0),
    unit           TEXT          NOT NULL DEFAULT 'km' CHECK (unit IN ('km', 'mile')),
    currency       CHAR(3)       NOT NULL DEFAULT 'USD',
    effective_date DATE          NOT NULL,

    created_by     UUID          NOT NULL REFERENCES users(id),
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_mil_lookup ON hrm_mileage_rates (org_id, effective_date DESC);

-- hrm_expense_lines.mileage_rate_id points at the rate a mileage line used.
-- Added after the table exists; ON DELETE SET NULL, so deleting a superseded
-- rate does not delete claim history.
ALTER TABLE hrm_expense_lines
    ADD CONSTRAINT fk_hrm_expl_mileage_rate
    FOREIGN KEY (mileage_rate_id) REFERENCES hrm_mileage_rates(id) ON DELETE SET NULL;

-- ------------------------------------------------------------
-- hrm_expense_policy_violations — warnings, never blocks
-- ------------------------------------------------------------
CREATE TABLE hrm_expense_policy_violations (
    id            UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    line_id       UUID          NOT NULL REFERENCES hrm_expense_lines(id) ON DELETE CASCADE,
    policy_id     UUID          REFERENCES hrm_expense_policies(id) ON DELETE SET NULL,

    -- Snapshotted so the warning still reads correctly after the policy that
    -- produced it is re-priced or deleted — the same reasoning as every other
    -- snapshot in Phase 7/8.
    category      TEXT          NOT NULL,
    max_amount    NUMERIC(15,2) NOT NULL,
    actual_amount NUMERIC(15,2) NOT NULL,
    message       TEXT          NOT NULL,

    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
    -- No updated_at — append-only, like hrm_loan_recovery_events.
);

CREATE INDEX idx_hrm_expviol_line ON hrm_expense_policy_violations (line_id);

COMMENT ON TABLE hrm_expense_policy_violations IS 'A recorded WARNING, never a block. Submitting an over-policy claim succeeds; the approver sees which policy was breached and by how much. A boolean on the line could say neither.';

-- ------------------------------------------------------------
-- Widen BOTH approval CHECKs — templates use the short vocabulary,
-- instances the long one. See 00098/00106 headers.
-- ------------------------------------------------------------
ALTER TABLE hrm_approval_templates
    DROP CONSTRAINT IF EXISTS hrm_approval_templates_action_type_check,
    ADD CONSTRAINT hrm_approval_templates_action_type_check
        CHECK (action_type IN (
            'leave', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'offer', 'custom',
            'salary_revision', 'bonus', 'loan', 'reimbursement', 'asset_request',
            'travel_request', 'expense_claim'
        ));

ALTER TABLE hrm_approval_instances
    DROP CONSTRAINT IF EXISTS hrm_approval_instances_entity_type_check,
    ADD CONSTRAINT hrm_approval_instances_entity_type_check
        CHECK (entity_type IN (
            'leave_request', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'offer', 'custom',
            'salary_revision', 'bonus', 'loan', 'reimbursement', 'asset_request',
            'travel_request', 'expense_claim'
        ));

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE hrm_approval_instances
    DROP CONSTRAINT IF EXISTS hrm_approval_instances_entity_type_check,
    ADD CONSTRAINT hrm_approval_instances_entity_type_check
        CHECK (entity_type IN (
            'leave_request', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'offer', 'custom',
            'salary_revision', 'bonus', 'loan', 'reimbursement', 'asset_request'
        ));

ALTER TABLE hrm_approval_templates
    DROP CONSTRAINT IF EXISTS hrm_approval_templates_action_type_check,
    ADD CONSTRAINT hrm_approval_templates_action_type_check
        CHECK (action_type IN (
            'leave', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'offer', 'custom',
            'salary_revision', 'bonus', 'loan', 'reimbursement', 'asset_request'
        ));

-- Reverse dependency order. Violations reference lines AND policies, so they
-- go first; lines reference mileage rates via an FK added after both tables
-- existed, so lines must go before rates. NOT the mirror of CREATE order —
-- 6B's 00094 shipped a broken Down block by assuming symmetry.
DROP TABLE IF EXISTS hrm_expense_policy_violations;
DROP TABLE IF EXISTS hrm_expense_lines;
DROP TABLE IF EXISTS hrm_mileage_rates;
DROP TABLE IF EXISTS hrm_per_diem_rates;
DROP TABLE IF EXISTS hrm_expense_policies;
DROP TABLE IF EXISTS hrm_expense_claims;
DROP TABLE IF EXISTS hrm_travel_advances;
DROP TABLE IF EXISTS hrm_travel_itinerary_items;
DROP TABLE IF EXISTS hrm_travel_requests;

-- +goose StatementEnd
