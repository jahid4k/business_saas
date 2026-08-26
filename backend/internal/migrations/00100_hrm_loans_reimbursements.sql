-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00100_hrm_loans_reimbursements
--
-- Phase 7C: employee loans with amortization, and reimbursement payout.
-- Four tables:
--
--   hrm_loans                — the loan itself; a decision, then a disbursement
--   hrm_loan_schedules        — the amortization schedule, generated ONCE
--   hrm_loan_recovery_events  — append-only ledger, one row per PARTIAL or
--                               full recovery against a schedule installment
--   hrm_reimbursements        — payout only, no claims workflow
--
-- Design notes:
--
--   • The amortization schedule is generated ONCE, at disbursement, and
--     THEREAFTER READ, NEVER RECOMPUTED by payroll. The same immutability
--     reasoning as 5B's published appraisals and 6A's frozen quiz grades: if
--     payroll re-derived the schedule from principal/rate/tenure on every
--     run, a later interest-rate correction or rounding-mode change would
--     silently rewrite what an employee was told they'd pay in month 3 after
--     they'd already paid months 1 and 2 under the old numbers.
--
--   • hrm_loan_schedules.recovered_amount and status ('pending' /
--     'partially_recovered' / 'recovered' / 'foreclosed') exist because a
--     single installment can be recovered ACROSS MULTIPLE PAYROLL RUNS: the
--     zero-net-pay guard caps how much of a due installment payroll may
--     recover in one run (recovery must not push net negative), so a
--     shortfall carries forward and is topped up next run. total_amount
--     itself never changes after generation — recovered_amount is the only
--     mutable field, and it only ever increases toward total_amount.
--
--   • hrm_loan_recovery_events is the audit ledger this makes necessary. A
--     single FK from hrm_loan_schedules to "the line that recovered it"
--     cannot represent a partial-then-partial-then-complete history across
--     three different runs. One row per actual recovery event, append-only —
--     the hrm_application_stage_history (00078) shape.
--
--   • hrm_reimbursements has NO calculation_snapshot. That column on
--     hrm_salary_revisions/hrm_bonuses (00098) is mandatory specifically
--     because it is the audit record of a shared CompensationContext-driven
--     FORMULA computation (band, compa-ratio, rating, matrix cell).
--     Reimbursement is "payout only" per the build plan — HR types a flat
--     amount for an expense the employee already incurred — there is no
--     formula to snapshot, and forcing one onto every reimbursement would
--     manufacture a JSONB blob nobody ever reads. Claims tracking (receipts,
--     line-item breakdown) is explicitly out of scope, per the build plan.
--
--   • Loans and reimbursements both widen the SAME two CHECK constraints
--     00098 already widened once for salary_revision/bonus — and both
--     constraints again, not just one. See that migration's header: template
--     action_type uses the short form, instance entity_type uses the long
--     form, and they are separate constraints on separate tables.
--
-- What must NEVER be added (the 00076 CHECK x ON DELETE SET NULL trap):
-- hrm_loan_recovery_events.payslip_run_id/payslip_line_id and
-- hrm_reimbursements.payslip_run_id/payslip_line_id are ON DELETE SET NULL.
-- A CHECK pairing either with another column (e.g. requiring
-- payslip_line_id whenever status='paid') would break DELETE on
-- hrm_payslip_runs/hrm_payslip_lines the same way 00096 already documented.
-- The service validates that pairing instead.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_loans
-- ------------------------------------------------------------
CREATE TABLE hrm_loans (
    id                    UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id             TEXT          NOT NULL UNIQUE
                                            DEFAULT ('ln_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id           UUID          NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    loan_type             TEXT          NOT NULL
                                            CHECK (loan_type IN ('personal', 'emergency', 'advance', 'other')),
    principal_amount      NUMERIC(15,2) NOT NULL CHECK (principal_amount > 0),
    interest_rate_pct     NUMERIC(5,2)  NOT NULL DEFAULT 0 CHECK (interest_rate_pct >= 0),
    tenure_months         INTEGER       NOT NULL CHECK (tenure_months > 0),
    -- The per-installment amount the amortization produces. NULL until
    -- disbursement generates the schedule — it is the schedule's own output,
    -- not an input, so it cannot be known before generation.
    installment_amount    NUMERIC(15,2),

    status                TEXT          NOT NULL DEFAULT 'draft'
                                            CHECK (status IN (
                                                'draft', 'pending_approval', 'approved',
                                                'active', 'foreclosed', 'completed',
                                                'rejected', 'cancelled'
                                            )),
    approval_instance_id  UUID          REFERENCES hrm_approval_instances(id) ON DELETE SET NULL,

    disbursed_at          TIMESTAMPTZ,
    disbursed_by          UUID          REFERENCES users(id),
    foreclosed_at         TIMESTAMPTZ,
    -- The lump sum that closed out the loan early. Distinct from the sum of
    -- remaining total_amount, because foreclosure commonly waives some
    -- portion of remaining interest — a business decision recorded here, not
    -- derived.
    foreclosure_amount    NUMERIC(15,2),

    created_by            UUID          NOT NULL REFERENCES users(id),
    created_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_ln_org_id      ON hrm_loans (org_id);
CREATE INDEX idx_hrm_ln_employee_id ON hrm_loans (employee_id);
CREATE INDEX idx_hrm_ln_status      ON hrm_loans (org_id, status);

COMMENT ON TABLE  hrm_loans IS 'An employee loan; the amortization schedule is generated once at disbursement and never recomputed';
COMMENT ON COLUMN hrm_loans.installment_amount IS 'Output of amortization at disbursement, not an input — NULL until then';

-- ------------------------------------------------------------
-- hrm_loan_schedules
-- ------------------------------------------------------------
CREATE TABLE hrm_loan_schedules (
    id                     UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id              TEXT          NOT NULL UNIQUE
                                             DEFAULT ('lns_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    loan_id                UUID          NOT NULL REFERENCES hrm_loans(id) ON DELETE CASCADE,

    installment_number     INTEGER       NOT NULL CHECK (installment_number > 0),
    due_period_year        INTEGER       NOT NULL,
    due_period_month       INTEGER       NOT NULL CHECK (due_period_month BETWEEN 1 AND 12),

    -- Frozen at generation. Never updated after the row is created — see
    -- migration header.
    principal_component    NUMERIC(15,2) NOT NULL CHECK (principal_component >= 0),
    interest_component     NUMERIC(15,2) NOT NULL CHECK (interest_component >= 0),
    total_amount           NUMERIC(15,2) NOT NULL CHECK (total_amount >= 0),

    -- The only mutable money on this row. Increases toward total_amount
    -- across however many payroll runs it takes to fully recover it.
    recovered_amount       NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (recovered_amount >= 0),
    status                 TEXT          NOT NULL DEFAULT 'pending'
                                             CHECK (status IN ('pending', 'partially_recovered', 'recovered', 'foreclosed')),

    created_at              TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    UNIQUE (loan_id, installment_number),
    CHECK (recovered_amount <= total_amount)
);

CREATE INDEX idx_hrm_lns_loan_id ON hrm_loan_schedules (loan_id, installment_number);
-- Backs "what is due for this employee's loans in this payroll period" —
-- the query the loan-recovery compute stage runs once per employee per run.
CREATE INDEX idx_hrm_lns_due ON hrm_loan_schedules (due_period_year, due_period_month)
    WHERE status IN ('pending', 'partially_recovered');

COMMENT ON TABLE hrm_loan_schedules IS 'Generated once at disbursement; recovered_amount is the only field ever updated after that, by payroll runs, never by re-amortizing';

-- ------------------------------------------------------------
-- hrm_loan_recovery_events
-- ------------------------------------------------------------
CREATE TABLE hrm_loan_recovery_events (
    id                UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    loan_id           UUID          NOT NULL REFERENCES hrm_loans(id) ON DELETE CASCADE,
    schedule_id       UUID          NOT NULL REFERENCES hrm_loan_schedules(id) ON DELETE CASCADE,
    payslip_run_id    UUID          REFERENCES hrm_payslip_runs(id) ON DELETE SET NULL,
    payslip_line_id   UUID          REFERENCES hrm_payslip_lines(id) ON DELETE SET NULL,
    amount_recovered  NUMERIC(15,2) NOT NULL CHECK (amount_recovered > 0),
    recovered_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
    -- No updated_at — append-only, exactly like hrm_application_stage_history.
);

CREATE INDEX idx_hrm_lre_schedule_id ON hrm_loan_recovery_events (schedule_id);
CREATE INDEX idx_hrm_lre_loan_id     ON hrm_loan_recovery_events (loan_id);

COMMENT ON TABLE hrm_loan_recovery_events IS 'Append-only ledger, one row per actual recovery event — an installment recovered across multiple runs (zero-net-pay capping) needs more than one row to audit';

-- ------------------------------------------------------------
-- hrm_reimbursements
-- ------------------------------------------------------------
CREATE TABLE hrm_reimbursements (
    id                    UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id             TEXT          NOT NULL UNIQUE
                                            DEFAULT ('rmb_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id           UUID          NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    category              TEXT          NOT NULL
                                            CHECK (category IN ('travel', 'medical', 'equipment', 'other')),
    description           TEXT,
    amount                NUMERIC(15,2) NOT NULL CHECK (amount > 0),
    currency              CHAR(3)       NOT NULL DEFAULT 'USD',

    status                TEXT          NOT NULL DEFAULT 'draft'
                                            CHECK (status IN (
                                                'draft', 'pending_approval', 'approved',
                                                'rejected', 'paid', 'cancelled'
                                            )),
    approval_instance_id  UUID          REFERENCES hrm_approval_instances(id) ON DELETE SET NULL,

    payslip_run_id        UUID          REFERENCES hrm_payslip_runs(id) ON DELETE SET NULL,
    payslip_line_id       UUID          REFERENCES hrm_payslip_lines(id) ON DELETE SET NULL,
    paid_at               TIMESTAMPTZ,

    created_by             UUID         NOT NULL REFERENCES users(id),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_rmb_org_id      ON hrm_reimbursements (org_id);
CREATE INDEX idx_hrm_rmb_employee_id ON hrm_reimbursements (employee_id);
CREATE INDEX idx_hrm_rmb_payable     ON hrm_reimbursements (org_id, status)
    WHERE status = 'approved';

COMMENT ON TABLE hrm_reimbursements IS 'Payout only — no claims/receipts workflow, no calculation_snapshot (there is no formula to audit, just a flat HR-entered amount). Paid out as a line_type=reimbursement earning line through any payroll run.';

-- ------------------------------------------------------------
-- Widen hrm_approval_templates.action_type AND
-- hrm_approval_instances.entity_type for loan/reimbursement.
-- ------------------------------------------------------------
ALTER TABLE hrm_approval_templates
    DROP CONSTRAINT IF EXISTS hrm_approval_templates_action_type_check,
    ADD CONSTRAINT hrm_approval_templates_action_type_check
        CHECK (action_type IN (
            'leave', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'offer', 'custom',
            'salary_revision', 'bonus', 'loan', 'reimbursement'
        ));

ALTER TABLE hrm_approval_instances
    DROP CONSTRAINT IF EXISTS hrm_approval_instances_entity_type_check,
    ADD CONSTRAINT hrm_approval_instances_entity_type_check
        CHECK (entity_type IN (
            'leave_request', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'offer', 'custom',
            'salary_revision', 'bonus', 'loan', 'reimbursement'
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
            'salary_revision', 'bonus'
        ));

ALTER TABLE hrm_approval_templates
    DROP CONSTRAINT IF EXISTS hrm_approval_templates_action_type_check,
    ADD CONSTRAINT hrm_approval_templates_action_type_check
        CHECK (action_type IN (
            'leave', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'offer', 'custom',
            'salary_revision', 'bonus'
        ));

DROP TABLE IF EXISTS hrm_reimbursements;
DROP TABLE IF EXISTS hrm_loan_recovery_events;
DROP TABLE IF EXISTS hrm_loan_schedules;
DROP TABLE IF EXISTS hrm_loans;

-- +goose StatementEnd
