-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00098_hrm_compensation
--
-- Phase 7B: salary revision cycles and the bonus engine. Five tables:
--
--   hrm_compensation_bands       — min/mid/max per grade, effective-dated
--   hrm_merit_matrix_cells       — rating × compa-ratio → increase %, effective-dated
--   hrm_salary_revision_cycles   — batch container, one approval per cycle
--   hrm_salary_revisions         — one row per employee within a cycle
--   hrm_bonuses                  — one row per employee bonus
--
-- Design notes:
--
--   • hrm_compensation_bands is MUTABLE catalog data despite being
--     effective-dated — unlike hrm_employee_salary_records (00023), a band is
--     configuration, not a decision. What must survive a later edit is the
--     compa-ratio computed against it AT CALCULATION TIME, and that is frozen
--     into hrm_salary_revisions.calculation_snapshot — the
--     hrm_approval_instances.instance_snapshot (00024) pattern. Editing a band
--     tomorrow does not rewrite a decision made against today's numbers.
--     grade_label is free TEXT with no FK, matching
--     hrm_salary_structures.grade_label (00023), which is also free TEXT.
--
--   • hrm_merit_matrix_cells is the same shape for the same reason: real,
--     queryable rows rather than JSONB, because computing a cycle means
--     MATCHING a rating level and a compa-ratio against ranges — exactly the
--     "slabs move from code into effective-dated rows" instruction the build
--     plan gives Phase 7D's statutory engine, applied here first.
--
--   • hrm_salary_revision_cycles carries ONE approval_instance_id for the
--     WHOLE cycle. The build plan calls this "batch-approved" — an approver
--     reviews and approves every proposed revision in the cycle in one
--     decision, not one approval chain per employee. entity_type =
--     'salary_revision', entity_id = the cycle's id (see the two CHECK
--     widenings below — hrm_approval_TEMPLATES.action_type uses the short
--     form 'leave'/'promotion'/etc., hrm_approval_INSTANCES.entity_type uses
--     'leave_request'/etc.; they are separate constraints on separate tables
--     with separate vocabularies and both must be widened, or a template can
--     be created for salary_revision/bonus but no instance of one).
--
--   • hrm_salary_revisions.calculation_snapshot is MANDATORY (NOT NULL, no
--     empty default) — the build plan requires it, and unlike
--     hrm_bonuses.calculation_snapshot the value here decides real pay: the
--     band matched, the compa-ratio computed against it, the rating level and
--     value read, and the matrix cell selected. A revision with an empty
--     snapshot is a revision nobody can audit.
--
--   • computation_warning (nullable TEXT) is deliberately a real column, not
--     folded into the snapshot. It flags "no compensation band for this
--     employee's grade" or "no published appraisal to rate against" — cases
--     where the engine could not compute a proposed amount and defaulted to
--     current pay. Burying that inside JSONB would make it invisible to any
--     query that lists a cycle's revisions needing attention.
--
--   • hrm_bonuses.calculation_snapshot is mandatory for the same reason the
--     build plan states directly: "shared CompensationContext builder feeding
--     both salary and bonus formulas ... calculation_snapshot JSONB
--     mandatory". A discretionary bonus with a hand-typed amount still
--     snapshots {"type": "discretionary", "amount": ...} — there is no
--     calc-method branch that skips it.
--
--   • hrm_bonuses.payslip_run_id / payslip_line_id are set once a bonus is
--     paid out through a run_type='bonus' payroll run (migration 00096).
--     Nullable and ON DELETE SET NULL: deleting the run must not delete the
--     bonus record, only sever the link back to how it was paid. This is
--     hrm_bonuses' real consumer — an approved bonus nothing ever pays is the
--     same defect class as the dropped hrm_employees.status column (r25):
--     a written record nothing reads.
--
--   • Neither hrm_compensation_bands nor hrm_merit_matrix_cells carries
--     employee_id, so neither is scope-tiered (see 00099's header) — the
--     Phase 1 tiers filter FROM hrm_employees and imply a per-employee filter
--     that cannot exist on catalog data. hrm_salary_revisions and hrm_bonuses
--     both carry employee_id and ARE scope-tiered.
--
-- What must NEVER be added (the 00076 CHECK × ON DELETE SET NULL trap):
-- source_period_id on hrm_payslip_lines (00096) is ON DELETE SET NULL, and so
-- are the FKs added here. A CHECK pairing any of them with another column
-- would break DELETE on the referenced table the same way, so none is added.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_compensation_bands
-- ------------------------------------------------------------
CREATE TABLE hrm_compensation_bands (
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id        TEXT          NOT NULL UNIQUE
                                        DEFAULT ('cb_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id           UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    grade_label      TEXT          NOT NULL,
    currency         CHAR(3)       NOT NULL DEFAULT 'USD',
    min_amount       NUMERIC(15,2) NOT NULL CHECK (min_amount >= 0),
    mid_amount       NUMERIC(15,2) NOT NULL CHECK (mid_amount >= 0),
    max_amount       NUMERIC(15,2) NOT NULL CHECK (max_amount >= 0),
    effective_date   DATE          NOT NULL,

    created_by       UUID          NOT NULL REFERENCES users(id),
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CHECK (min_amount <= mid_amount AND mid_amount <= max_amount)
);

CREATE INDEX idx_hrm_cb_org_id ON hrm_compensation_bands (org_id);
CREATE INDEX idx_hrm_cb_lookup ON hrm_compensation_bands (org_id, LOWER(grade_label), effective_date DESC);

COMMENT ON TABLE  hrm_compensation_bands IS 'Min/mid/max pay per grade; the row with the latest effective_date <= reference date is active. Mutable catalog data — see migration header';
COMMENT ON COLUMN hrm_compensation_bands.mid_amount IS 'Compa-ratio = employee basic_pay / mid_amount of their matched band; computed, never stored';

-- ------------------------------------------------------------
-- hrm_merit_matrix_cells
-- ------------------------------------------------------------
CREATE TABLE hrm_merit_matrix_cells (
    id                UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id         TEXT          NOT NULL UNIQUE
                                        DEFAULT ('mmc_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id            UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    rating_level_id   UUID          NOT NULL REFERENCES hrm_rating_scale_levels(id) ON DELETE CASCADE,

    -- Compa-ratio range this cell applies to. 1.00 = paid exactly at band
    -- midpoint. compa_ratio_max NULL means no upper bound (open-ended).
    compa_ratio_min   NUMERIC(6,4)  NOT NULL CHECK (compa_ratio_min >= 0),
    compa_ratio_max   NUMERIC(6,4)  CHECK (compa_ratio_max IS NULL OR compa_ratio_max > compa_ratio_min),

    increase_pct      NUMERIC(5,2)  NOT NULL,
    effective_date    DATE          NOT NULL,

    created_by        UUID          NOT NULL REFERENCES users(id),
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_mmc_org_id ON hrm_merit_matrix_cells (org_id);
CREATE INDEX idx_hrm_mmc_lookup ON hrm_merit_matrix_cells (org_id, rating_level_id, effective_date DESC);

COMMENT ON TABLE hrm_merit_matrix_cells IS 'Rating level x compa-ratio range -> increase %. Real rows, not JSONB, because computing a cycle means matching ranges — the 00023/7D "slabs as rows" pattern applied to merit increases';

-- ------------------------------------------------------------
-- hrm_salary_revision_cycles
-- ------------------------------------------------------------
CREATE TABLE hrm_salary_revision_cycles (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id             TEXT        NOT NULL UNIQUE
                                          DEFAULT ('src_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name                  TEXT        NOT NULL,
    description           TEXT,
    -- The date approved revisions take effect — written as
    -- hrm_employee_salary_records.effective_date on apply.
    effective_date        DATE        NOT NULL,

    status                TEXT        NOT NULL DEFAULT 'draft'
                                          CHECK (status IN (
                                              'draft', 'computed', 'pending_approval',
                                              'approved', 'applied', 'rejected', 'cancelled'
                                          )),
    approval_instance_id  UUID        REFERENCES hrm_approval_instances(id) ON DELETE SET NULL,

    created_by            UUID        NOT NULL REFERENCES users(id),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    computed_at           TIMESTAMPTZ,
    submitted_at          TIMESTAMPTZ,
    applied_at            TIMESTAMPTZ,
    applied_by            UUID        REFERENCES users(id)
);

CREATE INDEX idx_hrm_src_org_id ON hrm_salary_revision_cycles (org_id, status);

COMMENT ON TABLE hrm_salary_revision_cycles IS 'Batch container for a compensation review round; one approval instance covers every revision in the cycle';

-- ------------------------------------------------------------
-- hrm_salary_revisions
-- ------------------------------------------------------------
CREATE TABLE hrm_salary_revisions (
    id                      UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id               TEXT          NOT NULL UNIQUE
                                              DEFAULT ('sr_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                  UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    cycle_id                UUID          NOT NULL REFERENCES hrm_salary_revision_cycles(id) ON DELETE CASCADE,
    employee_id             UUID          NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    current_basic_pay       NUMERIC(15,2) NOT NULL,
    proposed_basic_pay      NUMERIC(15,2) NOT NULL CHECK (proposed_basic_pay >= 0),
    is_excluded             BOOLEAN       NOT NULL DEFAULT FALSE,

    rating_level_id         UUID          REFERENCES hrm_rating_scale_levels(id) ON DELETE SET NULL,
    -- Mandatory — see migration header. What the merit engine matched against:
    -- band, compa-ratio, rating value, matrix cell, or a manual override.
    calculation_snapshot    JSONB         NOT NULL,
    -- Set only when the engine could not compute a proposed amount and left
    -- proposed_basic_pay == current_basic_pay by default. A real column so it
    -- is queryable, not buried inside the snapshot — see migration header.
    computation_warning     TEXT,
    override_reason         TEXT,

    -- Set once the cycle is applied and this row's revision has been written
    -- as a real salary record.
    salary_record_id        UUID          REFERENCES hrm_employee_salary_records(id) ON DELETE SET NULL,

    created_at              TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    UNIQUE (cycle_id, employee_id)
);

CREATE INDEX idx_hrm_sr_org_id      ON hrm_salary_revisions (org_id);
CREATE INDEX idx_hrm_sr_cycle_id    ON hrm_salary_revisions (cycle_id);
CREATE INDEX idx_hrm_sr_employee_id ON hrm_salary_revisions (employee_id);

COMMENT ON TABLE  hrm_salary_revisions IS 'One proposed (then applied) revision per employee within a cycle; pct_increase is computed from current/proposed at read time, never stored';
COMMENT ON COLUMN hrm_salary_revisions.calculation_snapshot IS 'Mandatory audit record of the CompensationContext used to derive proposed_basic_pay — band, compa-ratio, rating, matrix cell';

-- ------------------------------------------------------------
-- hrm_bonuses
-- ------------------------------------------------------------
CREATE TABLE hrm_bonuses (
    id                     UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id              TEXT          NOT NULL UNIQUE
                                             DEFAULT ('bns_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                 UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id            UUID          NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    bonus_type             TEXT          NOT NULL
                                             CHECK (bonus_type IN (
                                                 'performance', 'discretionary', 'signing',
                                                 'retention', 'referral', 'other'
                                             )),
    description            TEXT,
    period_year            INTEGER       NOT NULL,
    -- Nullable: an annual bonus need not tie to one payroll month the way a
    -- payslip run does.
    period_month           INTEGER       CHECK (period_month IS NULL OR period_month BETWEEN 1 AND 12),

    amount                 NUMERIC(15,2) NOT NULL CHECK (amount >= 0),
    currency                CHAR(3)      NOT NULL DEFAULT 'USD',
    -- Mandatory — see migration header.
    calculation_snapshot    JSONB        NOT NULL,

    status                  TEXT         NOT NULL DEFAULT 'draft'
                                             CHECK (status IN (
                                                 'draft', 'pending_approval', 'approved',
                                                 'rejected', 'paid', 'cancelled'
                                             )),
    approval_instance_id    UUID         REFERENCES hrm_approval_instances(id) ON DELETE SET NULL,

    -- Set once paid out through a run_type='bonus' payroll run (00096).
    payslip_run_id          UUID         REFERENCES hrm_payslip_runs(id) ON DELETE SET NULL,
    payslip_line_id         UUID         REFERENCES hrm_payslip_lines(id) ON DELETE SET NULL,
    paid_at                 TIMESTAMPTZ,

    created_by              UUID         NOT NULL REFERENCES users(id),
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_bns_org_id      ON hrm_bonuses (org_id);
CREATE INDEX idx_hrm_bns_employee_id ON hrm_bonuses (employee_id);
-- Backs "which approved bonuses are still owed for this period", the query a
-- bonus payroll run uses to pull its lines.
CREATE INDEX idx_hrm_bns_payable ON hrm_bonuses (org_id, period_year, period_month)
    WHERE status = 'approved';

COMMENT ON TABLE  hrm_bonuses IS 'One bonus per employee; paid out through a run_type=bonus payroll run, which is this table''s real consumer — see migration header';
COMMENT ON COLUMN hrm_bonuses.calculation_snapshot IS 'Mandatory audit record of the CompensationContext used to derive amount, even for a hand-typed discretionary bonus';

-- ------------------------------------------------------------
-- Widen hrm_approval_templates.action_type AND
-- hrm_approval_instances.entity_type for the two new consumers. Both are
-- widened together — see migration header.
-- ------------------------------------------------------------
ALTER TABLE hrm_approval_templates
    DROP CONSTRAINT IF EXISTS hrm_approval_templates_action_type_check,
    ADD CONSTRAINT hrm_approval_templates_action_type_check
        CHECK (action_type IN (
            'leave', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'offer', 'custom',
            'salary_revision', 'bonus'
        ));

ALTER TABLE hrm_approval_instances
    DROP CONSTRAINT IF EXISTS hrm_approval_instances_entity_type_check,
    ADD CONSTRAINT hrm_approval_instances_entity_type_check
        CHECK (entity_type IN (
            'leave_request', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'offer', 'custom',
            'salary_revision', 'bonus'
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
            'attendance_regularization', 'award', 'job_requisition', 'offer', 'custom'
        ));

ALTER TABLE hrm_approval_templates
    DROP CONSTRAINT IF EXISTS hrm_approval_templates_action_type_check,
    ADD CONSTRAINT hrm_approval_templates_action_type_check
        CHECK (action_type IN (
            'leave', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'offer', 'custom'
        ));

DROP TABLE IF EXISTS hrm_bonuses;
DROP TABLE IF EXISTS hrm_salary_revisions;
DROP TABLE IF EXISTS hrm_salary_revision_cycles;
DROP TABLE IF EXISTS hrm_merit_matrix_cells;
DROP TABLE IF EXISTS hrm_compensation_bands;

-- +goose StatementEnd
