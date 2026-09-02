-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00114_hrm_exits
--
-- Phase 9A: the exit umbrella. Three tables:
--
--   hrm_exits                 — the exit PROCESS over an existing decision
--   hrm_exit_clearance_items  — the money overlay on an offboarding checklist
--   hrm_rehire_eligibility    — the org's rehire decision, finally readable
--
-- ⚠ hrm_exits IS AN UMBRELLA, NOT A REPLACEMENT. hrm_resignations (00033)
-- and hrm_terminations (00034) already own the DECISION to end employment,
-- including its approval chain, its dates and its letter. This table owns
-- the PROCESS that follows: clearance, settlement, documents, access. A
-- migration that folded those two into this one would throw away two
-- shipped approval flows to gain nothing.
--
-- Design notes:
--
--   • source_type/source_id are POLYMORPHIC AND FK-FREE, the fourth
--     instance of a pattern this codebase has now settled on:
--     platform_checklist_instances.subject_type (00076),
--     platform_form_instances.subject_type (00084) and
--     platform_tickets.requester_type (00110). Adding 'abandonment' or
--     'end_of_contract' later is a CHECK widening, not a rewrite, and no
--     partial FK has to be untangled first. The trade is that referential
--     integrity for source_id is the service's job — stated here so nobody
--     "fixes" it into an FK later.
--
--   • THERE IS NO hrm_notice_periods TABLE, though the build plan named one.
--     hrm_resignations already carries notice_period_days, is_notice_waived
--     and last_working_date, snapshotted from the active contract at
--     submission. A second table holding the same three facts is a second
--     source of truth, and the first divergence between them is a payroll
--     bug nobody can adjudicate. What genuinely does not exist is the
--     SHORTFALL — an employee who leaves before serving notice — so that is
--     the one new column here.
--
--   • notice_shortfall_days STORES DAYS, NOT MONEY. Converting days to a
--     recoverable amount needs a daily rate, which belongs to the salary
--     structure and changes independently of this record. Phase 9B computes
--     the money at settlement time. Storing an amount here would freeze a
--     rate at the wrong moment and silently disagree with the payslip.
--
--   • CLEARANCE COMPLETION IS NOT A COLUMN. It is "every blocking clearance
--     item is resolved", derived from the checklist instance on read — the
--     00076 rule. hrm_terminations.exit_clearance_completed (00034) is
--     exactly the denormalized boolean this avoids; it stays where it is as
--     a legacy column and Phase 9 neither reads nor writes it. An
--     integration test introspects information_schema to prove no
--     clearance_completed / is_cleared / blocking_total column appears here.
--
-- What must NEVER be added (the 00076 CHECK x ON DELETE SET NULL trap):
-- Postgres re-evaluates CHECKs on UPDATE and ON DELETE SET NULL *is* an
-- UPDATE. checklist_instance_id and fnf_payslip_run_id are both ON DELETE
-- SET NULL, so:
--   • CHECK (status <> 'settled' OR fnf_payslip_run_id IS NOT NULL) would
--     make DELETE FROM hrm_payslip_runs fail 23514 for any settled exit.
--   • CHECK (checklist_instance_id IS NOT NULL OR status = 'initiated')
--     pairs two nullable columns and fails the same way.
-- The service validates both transitions instead.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_exits
-- ------------------------------------------------------------
CREATE TABLE hrm_exits (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id               TEXT        NOT NULL UNIQUE
                                            DEFAULT ('exit_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                  UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id             UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE RESTRICT,

    -- The decision this process follows from. Polymorphic and FK-free; see
    -- the migration header.
    source_type             TEXT        NOT NULL
                                            CHECK (source_type IN ('resignation', 'termination')),
    source_id               UUID        NOT NULL,

    -- Snapshotted from the source decision at exit creation, because the
    -- source's own dates may later be corrected and a settlement already
    -- computed against the old ones must remain explicable.
    last_working_date       DATE        NOT NULL,
    -- What the notice period entitled the employer to. NULL when notice was
    -- waived: a waiver is not a shortfall, and conflating them bills a
    -- departing employee for time the company agreed to forgo.
    expected_last_working_date DATE,
    -- expected - actual, floored at zero, computed once at exit creation.
    -- Days only — Phase 9B turns this into money. See header.
    notice_shortfall_days   INTEGER     NOT NULL DEFAULT 0
                                            CHECK (notice_shortfall_days >= 0),

    -- The offboarding checklist driving clearance. Nullable because an exit
    -- is created before its checklist is instantiated, and because an org
    -- with no offboarding template still needs exits to work.
    checklist_instance_id   UUID        REFERENCES platform_checklist_instances(id) ON DELETE SET NULL,
    -- The F&F run settling this exit. Set by Phase 9B.
    fnf_payslip_run_id      UUID        REFERENCES hrm_payslip_runs(id) ON DELETE SET NULL,

    status                  TEXT        NOT NULL DEFAULT 'initiated'
                                            CHECK (status IN (
                                                'initiated', 'in_clearance', 'pending_settlement',
                                                'settled', 'completed', 'cancelled'
                                            )),

    -- Stamped by the Phase 9C scheduler sweep. Nullable, and its absence on
    -- a past last_working_date is exactly what the sweep looks for.
    access_revoked_at       TIMESTAMPTZ,

    remarks                 TEXT,
    created_by              UUID        NOT NULL REFERENCES users(id),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_exit_org_status ON hrm_exits (org_id, status);
CREATE INDEX idx_hrm_exit_employee   ON hrm_exits (org_id, employee_id);
CREATE INDEX idx_hrm_exit_source     ON hrm_exits (org_id, source_type, source_id);
-- Backs the Phase 9C access-revocation sweep: exits whose last working date
-- has passed and which have not yet been revoked.
CREATE INDEX idx_hrm_exit_revocation_due ON hrm_exits (last_working_date)
    WHERE access_revoked_at IS NULL AND status NOT IN ('cancelled');

-- One live exit per employee. A second exit for someone already leaving is a
-- data-entry mistake, and two exits would each compute their own settlement.
-- Partial, so a rehired employee who later leaves again gets a new one.
CREATE UNIQUE INDEX uq_hrm_exit_active ON hrm_exits (employee_id)
    WHERE status NOT IN ('completed', 'cancelled');

COMMENT ON TABLE hrm_exits IS 'The exit PROCESS over an existing resignation/termination decision. source_type/source_id are polymorphic and FK-free. There is deliberately NO clearance_completed column — completion is derived from the checklist instance.';
COMMENT ON COLUMN hrm_exits.notice_shortfall_days IS 'Days of notice not served, floored at zero. DAYS, not money — Phase 9B applies the daily rate at settlement time.';

-- ------------------------------------------------------------
-- hrm_exit_clearance_items
--
-- The thin overlay the checklist engine cannot express. Everything about
-- WHO owns a clearance step, whether it blocks, and whether it is done
-- already lives on platform_checklist_instance_items (00076: owner_type,
-- is_blocking, status). What it has no concept of is MONEY — and clearance
-- is where a department says "he still owes us 40,000 for the laptop".
-- Onboarding has no equivalent, which is why this is not on the engine.
--
-- This table is the seam between clearance and F&F: Phase 9B reads
-- blocking_amount as a settlement debit.
-- ------------------------------------------------------------
CREATE TABLE hrm_exit_clearance_items (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id           TEXT        NOT NULL UNIQUE
                                        DEFAULT ('exclr_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    exit_id             UUID        NOT NULL REFERENCES hrm_exits(id) ON DELETE CASCADE,

    -- The checklist item this annotates. ON DELETE SET NULL rather than
    -- CASCADE: a recorded debt outlives the checklist step that raised it,
    -- and deleting the step must not quietly forgive the money.
    checklist_item_id   UUID        REFERENCES platform_checklist_instance_items(id) ON DELETE SET NULL,

    department          TEXT        NOT NULL,
    description         TEXT        NOT NULL,
    -- What this department says is outstanding. Zero is the ordinary case —
    -- most clearance steps are "hand back the badge", not a debt.
    blocking_amount     NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (blocking_amount >= 0),
    currency            TEXT        NOT NULL DEFAULT 'BDT',

    -- Resolved means the department is satisfied: either the item was
    -- returned/settled, or the amount was waived. An unresolved item with a
    -- non-zero amount is what Phase 9B charges.
    is_resolved         BOOLEAN     NOT NULL DEFAULT FALSE,
    resolved_by         UUID        REFERENCES users(id) ON DELETE SET NULL,
    resolved_at         TIMESTAMPTZ,
    resolution_note     TEXT,

    created_by          UUID        NOT NULL REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_exclr_exit ON hrm_exit_clearance_items (exit_id, is_resolved);

COMMENT ON COLUMN hrm_exit_clearance_items.blocking_amount IS 'Money this department says is outstanding. Read by Phase 9B as an F&F debit when the item is unresolved.';

-- ------------------------------------------------------------
-- hrm_rehire_eligibility
--
-- ⚠ hrm_terminations.is_rehire_eligible has existed since migration 00034,
-- commented "FALSE for gross misconduct; blocks future rehire in HR tools",
-- and NOTHING HAS EVER READ IT. Same shape as hrm_salary_components.is_taxable,
-- which sat unread from 00023 until Phase 7D needed it.
--
-- It is not enough on its own for two reasons: a resignation can also carry
-- a do-not-rehire decision and terminations are only half the exits, and a
-- boolean cannot record WHY or WHO decided — which is the part a recruiter
-- looking at a flagged candidate actually needs. This table is the readable
-- decision; the exit seeds it from is_rehire_eligible when the source is a
-- termination, so the old column becomes an input rather than dead weight.
-- ------------------------------------------------------------
CREATE TABLE hrm_rehire_eligibility (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT        NOT NULL UNIQUE
                                    DEFAULT ('rhe_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id          UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id     UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,
    exit_id         UUID        REFERENCES hrm_exits(id) ON DELETE SET NULL,

    status          TEXT        NOT NULL DEFAULT 'eligible'
                                    CHECK (status IN ('eligible', 'not_eligible', 'conditional')),
    reason          TEXT,
    decided_by      UUID        REFERENCES users(id) ON DELETE SET NULL,
    decided_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One standing decision per employee. A later exit UPDATEs it rather than
-- inserting a second — a recruiter asking "may we rehire this person" needs
-- one answer, not a history to adjudicate.
CREATE UNIQUE INDEX uq_hrm_rhe_employee ON hrm_rehire_eligibility (employee_id);
CREATE INDEX idx_hrm_rhe_org ON hrm_rehire_eligibility (org_id, status);

COMMENT ON TABLE hrm_rehire_eligibility IS 'The org standing decision on rehiring a former employee. Gives hrm_terminations.is_rehire_eligible (00034) its first reader. Checked by recruitment on candidate create as a WARNING, never a hard block.';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Reverse dependency order: clearance items and rehire rows both reference
-- hrm_exits, so exits goes LAST. Not the mirror of CREATE order — 6B's
-- 00094 shipped a broken Down block assuming symmetry.
DROP TABLE IF EXISTS hrm_exit_clearance_items;
DROP TABLE IF EXISTS hrm_rehire_eligibility;
DROP TABLE IF EXISTS hrm_exits;

-- +goose StatementEnd
