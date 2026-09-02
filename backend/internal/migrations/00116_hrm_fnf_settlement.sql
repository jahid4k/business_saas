-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00116_hrm_fnf_settlement
--
-- Phase 9B: full & final settlement. Two tables:
--
--   hrm_gratuity_rules          — effective-dated tenure gratuity
--   hrm_exit_settlement_lines   — the audit trail of every credit/debit
--
-- ⚠ F&F IS AN OFF-CYCLE PAYROLL RUN, NOT A SEPARATE CALCULATOR. The build
-- plan is emphatic and the code follows it: run_type='fnf' (which has existed
-- unused in payslips.RunType since 00096) reuses the same line types, the
-- same statutory engine and the same immutability as every other run. There
-- is no parallel settlement calculator here, and there must never be one —
-- two engines computing pay is how the two disagree.
--
-- ⚠ AND IT IS THE 'ADDS-ON' INTEGRATION SHAPE, NOT 'REPLACES'.
-- computeBonusPayslips REPLACES the salary computation because a bonus run
-- must not pay regular salary. F&F MUST pay prorated final salary — it is
-- the largest credit in most settlements — so it reuses the ordinary
-- per-employee computation with the EMPLOYEE SET narrowed to the leaver, and
-- appends its own credits/debits exactly as loans, reimbursements, statutory
-- and benefits already do.
--
-- Design notes:
--
--   • NEGATIVE NET IS A VALID OUTCOME HERE, and only here. ApproveRun
--     refuses any run containing a negative payslip — an r25 money-defect fix
--     that stays exactly as it is for regular/off_cycle/bonus/arrears. An
--     F&F run is the one case where deductions exceeding gross is a real
--     answer rather than a data problem: the employee owes the company on the
--     way out. The guard becomes run-type-aware; it is not removed.
--
--   • hrm_exit_settlement_lines IS AN AUDIT TRAIL, NOT THE MONEY. The
--     payslip and its lines remain the single source of truth for what was
--     paid. This table records WHERE each figure came from — which loan,
--     which clearance item, which advance — because six months later
--     "recovered 40,000" is unanswerable without it. Same reasoning as
--     hrm_loan_recovery_events (00100).
--
--   • GRATUITY IS EFFECTIVE-DATED, reusing 7D's SlabsAsOf shape
--     (MAX(effective_date) <= asOf, 00102): a rule revised next month must
--     not alter a settlement computed this month. A rule is revised by
--     inserting a new row, never by editing one in place.
--
--   • THE TRAINING BOND IS DELIBERATELY NOT BUILT, though the build plan
--     lists it as an F&F debit. Phase 6 (Learning) never created any
--     substrate for it — no course cost, no bond agreement, no recovery
--     schedule — so building it here means inventing a Learning feature
--     inside an Exit phase. Recorded so the omission reads as a decision
--     rather than an oversight.
--
-- What must NEVER be added (the 00076 CHECK x ON DELETE SET NULL trap):
-- Postgres re-evaluates CHECKs on UPDATE and ON DELETE SET NULL *is* an
-- UPDATE. payslip_line_id is ON DELETE SET NULL, so
--   CHECK (payslip_line_id IS NOT NULL OR is_credit = FALSE)
-- would make DELETE FROM hrm_payslip_lines fail 23514 for any settled exit.
-- The service validates instead.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_gratuity_rules
-- ------------------------------------------------------------
CREATE TABLE hrm_gratuity_rules (
    id                    UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id             TEXT          NOT NULL UNIQUE
                                            DEFAULT ('grt_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name                  TEXT          NOT NULL,

    -- Tenure floor. Below this an employee is simply not entitled — zero, not
    -- an error, because "you did not qualify" is a normal settlement outcome.
    min_years_of_service  NUMERIC(5,2)  NOT NULL DEFAULT 5 CHECK (min_years_of_service >= 0),
    -- Days of pay awarded per completed year of service.
    days_per_year         NUMERIC(6,2)  NOT NULL CHECK (days_per_year > 0),
    -- Which figure the daily rate is derived from. 'basic' is the common
    -- statutory basis; 'gross' exists because some orgs contract on it.
    base_component        TEXT          NOT NULL DEFAULT 'basic'
                                            CHECK (base_component IN ('basic', 'gross')),
    -- The divisor turning a monthly figure into a daily one. A POLICY
    -- CHOICE, not an obvious fact: 30 is the common statutory convention,
    -- 26 excludes weekly offs, and getting it wrong changes every payout.
    -- Stored per rule so the choice is explicit and auditable.
    monthly_divisor       NUMERIC(5,2)  NOT NULL DEFAULT 30 CHECK (monthly_divisor > 0),

    -- Whether a for-cause termination forfeits gratuity. Off by default:
    -- forfeiture is a legally loaded decision and must be opted into.
    forfeit_on_misconduct BOOLEAN       NOT NULL DEFAULT FALSE,

    -- Effective-dated. A revision inserts a new row; the settlement reads
    -- whichever row was in force on the last working date.
    effective_date        DATE          NOT NULL,

    created_by            UUID          NOT NULL REFERENCES users(id),
    created_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_grt_org ON hrm_gratuity_rules (org_id, effective_date DESC);

COMMENT ON TABLE hrm_gratuity_rules IS 'Effective-dated tenure gratuity, read with the SlabsAsOf shape (MAX(effective_date) <= asOf) so a future revision cannot alter a settlement already computed.';
COMMENT ON COLUMN hrm_gratuity_rules.monthly_divisor IS 'Monthly-to-daily divisor. A policy choice (30 statutory vs 26 excluding weekly offs), stored per rule so it is explicit rather than buried in code.';

-- ------------------------------------------------------------
-- hrm_exit_settlement_lines
--
-- One row per credit or debit that fed the F&F payslip, naming its origin.
-- The payslip is the money; this explains it.
-- ------------------------------------------------------------
CREATE TABLE hrm_exit_settlement_lines (
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id        TEXT          NOT NULL UNIQUE
                                       DEFAULT ('esl_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    exit_id          UUID          NOT NULL REFERENCES hrm_exits(id) ON DELETE CASCADE,

    -- Polymorphic and FK-FREE, for the same reason hrm_exits.source_id is:
    -- the origin may be a loan, a clearance item, an advance or a computed
    -- figure with no row at all (gratuity, notice shortfall). A partial FK
    -- to four different tables is not expressible, and a nullable FK per
    -- source type would be four mostly-empty columns.
    source_type      TEXT          NOT NULL
                                       CHECK (source_type IN (
                                           'leave_encashment', 'gratuity', 'notice_shortfall',
                                           'loan_foreclosure', 'travel_advance',
                                           'clearance_due', 'other'
                                       )),
    source_id        UUID,
    description      TEXT          NOT NULL,

    -- Always POSITIVE; direction lives in is_credit. Storing debits as
    -- negatives means every reader has to know the sign convention, and the
    -- first one who does not produces a settlement that adds up backwards.
    amount           NUMERIC(15,2) NOT NULL CHECK (amount >= 0),
    is_credit        BOOLEAN       NOT NULL,
    currency         TEXT          NOT NULL DEFAULT 'BDT',

    -- The payslip line this became, when it became one. Nullable because a
    -- line can be computed for a draft settlement before any payslip exists.
    payslip_line_id  UUID          REFERENCES hrm_payslip_lines(id) ON DELETE SET NULL,

    created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW()
    -- No updated_at: append-only. A correction is a new run, not an edit —
    -- the hrm_loan_recovery_events discipline.
);

CREATE INDEX idx_hrm_esl_exit   ON hrm_exit_settlement_lines (exit_id, is_credit);
CREATE INDEX idx_hrm_esl_source ON hrm_exit_settlement_lines (source_type, source_id)
    WHERE source_id IS NOT NULL;

COMMENT ON TABLE hrm_exit_settlement_lines IS 'Append-only audit trail naming the origin of every F&F credit and debit. The payslip is the money; this explains where each figure came from. amount is ALWAYS positive — direction is is_credit.';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Settlement lines reference hrm_exits and hrm_payslip_lines but nothing
-- references them, so they drop first; gratuity rules are independent.
DROP TABLE IF EXISTS hrm_exit_settlement_lines;
DROP TABLE IF EXISTS hrm_gratuity_rules;

-- +goose StatementEnd
