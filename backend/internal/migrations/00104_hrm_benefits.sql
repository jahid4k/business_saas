-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00104_hrm_benefits
--
-- Phase 7D, second half: benefits administration. Four tables:
--
--   hrm_benefit_plans        — catalog: what plans exist (health, dental, ...)
--   hrm_benefit_tiers        — coverage levels within a plan, with cost
--   hrm_benefit_enrollments  — an employee's enrollment in one tier
--   hrm_dependents           — people covered under an enrollment
--
-- Design notes:
--
--   • hrm_benefit_tiers.employee_cost / employer_cost are MUTABLE catalog
--     data, exactly like 00098's hrm_compensation_bands — an org re-prices
--     a tier for the next plan year without rewriting what an already
--     enrolled employee is paying today. What must survive that repricing
--     is captured on the enrollment: employee_cost_snapshot /
--     employer_cost_snapshot, frozen at enrollment time. Same immutable-
--     decision-vs-mutable-catalog split as compensation bands vs. revisions.
--
--   • employer_cost_snapshot is recorded but does NOT produce a payslip
--     line. Only the employee's own cost does (a deduction, via
--     payslips.BenefitsSource — the loans/reimbursements consumer-owned
--     interface pattern, applied here). There is no consumer today for an
--     employer-cost payslip line — nothing reads or reports it — so adding
--     one now would be exactly the speculative primitive rule 1 forbids.
--     The column exists for the employer-cost REPORTING surface that is a
--     real, named future consumer (Section 9's compensation-analytics line),
--     not invented ahead of it.
--
--   • Benefit deductions use line_type='deduction', not a new line_type.
--     7A's six line_types (earning/deduction/arrear/reimbursement/
--     loan_recovery/statutory) are the build plan's own closed list — a
--     benefit premium needs no structural distinction downstream (nothing
--     joins on "this deduction came from benefits" the way loan recovery
--     lines are joined back to hrm_loan_schedules), so it is a plain
--     deduction. Inventing a seventh line_type here without a consumer that
--     needs it would be scope creep on 7A's own design, not a fix.
--
--   • "Enrollment windows" per the build plan means the THREE named window
--     TYPES (open / new-hire / qualifying-event) an enrollment can be made
--     under — modelled as enrollment_window_type on the enrollment row
--     itself, not a separate window-definition table. No such table shape
--     is asked for by the build plan, and inventing an org-configurable
--     "enrollment period" concept with its own start/end dates and
--     eligibility rules ahead of any request for one would be the
--     speculative-primitive trap again.
--
--   • hrm_dependents.enrollment_id is nullable: a dependent can be recorded
--     against a specific enrollment (the common case) or added ahead of one
--     being finalized. is_verified is a plain boolean with verified_by/at —
--     "manually verified, no verification workflow engine" is the build
--     plan's own words; a full document-upload/review pipeline is
--     explicitly out of scope.
--
-- What must NEVER be added (the 00076 CHECK x ON DELETE SET NULL trap):
-- hrm_dependents.enrollment_id is ON DELETE SET NULL. A CHECK requiring it
-- NOT NULL under any condition would break DELETE on hrm_benefit_enrollments
-- the same way every prior migration in this phase has documented.
-- ============================================================

CREATE TABLE hrm_benefit_plans (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id    TEXT        NOT NULL UNIQUE
                                 DEFAULT ('bfp_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id       UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name         TEXT        NOT NULL,
    plan_type    TEXT        NOT NULL
                                 CHECK (plan_type IN ('health', 'dental', 'vision', 'life', 'retirement', 'other')),
    description  TEXT,
    is_active    BOOLEAN     NOT NULL DEFAULT TRUE,

    created_by   UUID        NOT NULL REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_bfp_org_id ON hrm_benefit_plans (org_id, is_active);

CREATE TABLE hrm_benefit_tiers (
    id             UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id      TEXT          NOT NULL UNIQUE
                                     DEFAULT ('bft_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    plan_id        UUID          NOT NULL REFERENCES hrm_benefit_plans(id) ON DELETE CASCADE,

    tier_name      TEXT          NOT NULL,
    employee_cost  NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (employee_cost >= 0),
    employer_cost  NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (employer_cost >= 0),
    is_active      BOOLEAN       NOT NULL DEFAULT TRUE,

    created_by      UUID         NOT NULL REFERENCES users(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_bft_plan_id ON hrm_benefit_tiers (plan_id, is_active);

COMMENT ON TABLE hrm_benefit_tiers IS 'Mutable catalog data — repricing a tier does not retroactively change an already-enrolled employee''s cost, which is frozen on hrm_benefit_enrollments';

CREATE TABLE hrm_benefit_enrollments (
    id                       UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id                TEXT          NOT NULL UNIQUE
                                               DEFAULT ('bfe_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                   UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id              UUID          NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,
    plan_id                  UUID          NOT NULL REFERENCES hrm_benefit_plans(id) ON DELETE RESTRICT,
    tier_id                  UUID          NOT NULL REFERENCES hrm_benefit_tiers(id) ON DELETE RESTRICT,

    enrollment_window_type   TEXT          NOT NULL
                                               CHECK (enrollment_window_type IN ('open', 'new_hire', 'qualifying_event')),
    status                   TEXT          NOT NULL DEFAULT 'pending'
                                               CHECK (status IN ('pending', 'active', 'waived', 'terminated')),
    effective_date           DATE          NOT NULL,
    end_date                 DATE,

    -- Frozen at enrollment — see migration header.
    employee_cost_snapshot   NUMERIC(15,2) NOT NULL,
    employer_cost_snapshot   NUMERIC(15,2) NOT NULL,

    created_by                UUID         NOT NULL REFERENCES users(id),
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CHECK (end_date IS NULL OR end_date >= effective_date)
);

CREATE INDEX idx_hrm_bfe_org_id      ON hrm_benefit_enrollments (org_id);
CREATE INDEX idx_hrm_bfe_employee_id ON hrm_benefit_enrollments (employee_id);
-- Backs "what does this employee owe this payroll period" and the
-- scheduler's pending -> active sweep.
CREATE INDEX idx_hrm_bfe_status ON hrm_benefit_enrollments (org_id, status);
-- One active enrollment per (employee, plan) at a time — waived/terminated
-- rows are excluded, so re-enrolling after waiving is unaffected.
CREATE UNIQUE INDEX uq_hrm_bfe_employee_plan_active
    ON hrm_benefit_enrollments (employee_id, plan_id)
    WHERE status IN ('pending', 'active');

COMMENT ON TABLE hrm_benefit_enrollments IS 'One employee''s enrollment in one benefit tier; employee_cost_snapshot feeds a recurring payroll deduction line (line_type=deduction) once status=active';

CREATE TABLE hrm_dependents (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT        NOT NULL UNIQUE
                                    DEFAULT ('dep_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id          UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id     UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,
    enrollment_id   UUID        REFERENCES hrm_benefit_enrollments(id) ON DELETE SET NULL,

    full_name       TEXT        NOT NULL,
    relationship    TEXT        NOT NULL
                                    CHECK (relationship IN ('spouse', 'child', 'domestic_partner', 'other')),
    date_of_birth   DATE,

    is_verified     BOOLEAN     NOT NULL DEFAULT FALSE,
    verified_by     UUID        REFERENCES users(id),
    verified_at     TIMESTAMPTZ,

    created_by       UUID       NOT NULL REFERENCES users(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_dep_org_id        ON hrm_dependents (org_id);
CREATE INDEX idx_hrm_dep_employee_id   ON hrm_dependents (employee_id);
CREATE INDEX idx_hrm_dep_enrollment_id ON hrm_dependents (enrollment_id) WHERE enrollment_id IS NOT NULL;

COMMENT ON TABLE hrm_dependents IS 'Manually verified — no document-upload/review workflow, per the build plan';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_dependents;
DROP TABLE IF EXISTS hrm_benefit_enrollments;
DROP TABLE IF EXISTS hrm_benefit_tiers;
DROP TABLE IF EXISTS hrm_benefit_plans;

-- +goose StatementEnd
