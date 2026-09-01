-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00123_hrm_succession
--
-- Phase 10B: succession planning. Five tables:
--
--   hrm_critical_positions      — which roles the organization cannot afford
--                                 to leave empty, and why
--   hrm_talent_assessments      — the 9-box: performance AND potential,
--                                 assessed on SEPARATE axes
--   hrm_succession_candidates   — nominations against a critical position
--   hrm_development_plans       — the subject-visible half
--   hrm_development_plan_items  — the actions a plan actually consists of
--
-- ⚠ POTENTIAL IS ASSESSED SEPARATELY AND IS NEVER DERIVED FROM PERFORMANCE.
--
-- This is the single most important rule in the slice. If potential were
-- computed from the appraisal rating, every employee would land on the
-- grid's diagonal and the 9-box would carry exactly as much information as
-- the rating alone — an expensive way to draw one number twice. The whole
-- point of the instrument is that a strong performer in a role they have
-- outgrown and a struggling performer who is new to a hard role are
-- DIFFERENT cases requiring different action.
--
-- The schema enforces the separation three ways:
--
--   • performance_band and potential_band are independent columns with
--     independent CHECKs; neither has a default derived from the other.
--   • potential_rationale is NOT NULL. A potential band with no stated
--     reason is a number somebody made up, and the plan's ban on
--     unexplainable scoring applies here first.
--   • performance_band carries its provenance (the appraisal it was read
--     from, plus a snapshot of the value) while potential_band carries
--     none, because there is no upstream record it could come from.
--
-- ⚠ THERE IS NO box_number COLUMN. The 9-box position is
-- f(performance_band, potential_band) and is computed in Go
-- (orgchart-style, the 00076 computed-not-stored rule). Storing it would
-- create a third value that can disagree with the two it is derived from.
--
-- ⚠ CONFIDENTIALITY IS TWO READ PATHS, NOT FIELD FILTERING.
--
-- The build plan assumed "Phase 1's field-level filtering" as the enabler.
-- That was never built. Instead, the confidential material
-- (hrm_talent_assessments, hrm_succession_candidates) and the
-- subject-visible material (hrm_development_plans + items) live in
-- SEPARATE TABLES reached by SEPARATE REPOSITORY METHODS returning
-- SEPARATE TYPES. The subject's query never selects a confidential column,
-- so there is nothing in memory to forget to strip. Same structure as 5C's
-- 360 anonymity, 8C's internal ticket comments and 9C's exit interviews.
--
--   • The FK direction is load-bearing: hrm_succession_candidates points AT
--     a development plan, never the reverse. A development_plan row that
--     referenced a nomination would tell the subject — who can read their
--     own plan — that they are a named successor for a specific position.
--
--   • hrm_development_plans deliberately has NO plan_type column. A type
--     value like 'succession_readiness' would leak the same fact through a
--     field the subject is allowed to read.
--
-- ⚠ FLIGHT RISK IS COMPUTED, NOT STORED — there is no table for it here.
--
-- The plan requires explainable, signal-based indicators and explicitly
-- excludes predictive scoring. Every signal it names is already derivable
-- from data this database holds: time since last promotion
-- (hrm_promotions), pay below band (hrm_employee_salary_records ->
-- hrm_salary_structures.grade_label -> hrm_compensation_bands), manager
-- churn (hrm_reporting_relationships, 10A) and appraisal decline
-- (hrm_appraisals.final_rating_value). Storing them would create rows that
-- go stale the moment any of those change, with nothing to detect it.
-- Deriving them guarantees each signal can state the query that produced
-- it, which is what "explainable" has to mean.
-- ============================================================

-- ── Critical positions ───────────────────────────────────────────────────────
-- Which roles matter, and what happens if one empties. This is the only part
-- of succession that is org DESIGN rather than a judgement about a named
-- person, which is why it is the only part a manager may read.
CREATE TABLE hrm_critical_positions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id           TEXT NOT NULL UNIQUE
                        DEFAULT 'critpos_' || replace(gen_random_uuid()::text, '-', ''),
    org_id              UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    position_id         UUID NOT NULL REFERENCES hrm_positions(id) ON DELETE CASCADE,

    criticality_level   TEXT NOT NULL DEFAULT 'high',
    vacancy_risk        TEXT NOT NULL DEFAULT 'medium',
    impact_of_vacancy   TEXT,

    identified_by       UUID NOT NULL REFERENCES users(id),
    review_due_date     DATE,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    deactivated_at      TIMESTAMPTZ,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_critpos_level CHECK (
        criticality_level IN ('mission_critical', 'high', 'moderate')
    ),
    CONSTRAINT chk_hrm_critpos_risk CHECK (
        vacancy_risk IN ('high', 'medium', 'low')
    )
);

-- One ACTIVE designation per position. A position designated twice would
-- split its nominations across two rows and make "who succeeds this role"
-- unanswerable; the partial predicate still allows a re-designation after a
-- previous one is retired.
CREATE UNIQUE INDEX uq_hrm_critpos_active_position
    ON hrm_critical_positions (org_id, position_id) WHERE is_active;
CREATE INDEX idx_hrm_critpos_org_id     ON hrm_critical_positions (org_id);
CREATE INDEX idx_hrm_critpos_position   ON hrm_critical_positions (position_id);
CREATE INDEX idx_hrm_critpos_review_due ON hrm_critical_positions (org_id, review_due_date)
    WHERE is_active AND review_due_date IS NOT NULL;

-- ── Talent assessments (the 9-box) ───────────────────────────────────────────
CREATE TABLE hrm_talent_assessments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id           TEXT NOT NULL UNIQUE
                        DEFAULT 'talasm_' || replace(gen_random_uuid()::text, '-', ''),
    org_id              UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id         UUID NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,
    as_of_date          DATE NOT NULL,

    -- Performance axis: read from an appraisal, and it says which one.
    performance_band            TEXT NOT NULL,
    performance_appraisal_id    UUID REFERENCES hrm_appraisals(id) ON DELETE SET NULL,
    performance_rating_snapshot NUMERIC(6,2),

    -- Potential axis: assessed here, with no upstream source by design.
    -- The rationale is NOT NULL because a potential band nobody can explain
    -- is exactly the unexplainable score this phase forbids.
    potential_band       TEXT NOT NULL,
    potential_rationale  TEXT NOT NULL,

    assessed_by         UUID NOT NULL REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_talasm_perf CHECK (performance_band IN ('low', 'medium', 'high')),
    CONSTRAINT chk_hrm_talasm_pot  CHECK (potential_band  IN ('low', 'medium', 'high')),
    CONSTRAINT chk_hrm_talasm_rationale CHECK (length(btrim(potential_rationale)) > 0)
);

-- One assessment per employee per as-of date; a re-assessment on the same
-- date is an edit, not a second opinion.
CREATE UNIQUE INDEX uq_hrm_talasm_employee_asof
    ON hrm_talent_assessments (org_id, employee_id, as_of_date);
CREATE INDEX idx_hrm_talasm_org_id   ON hrm_talent_assessments (org_id);
CREATE INDEX idx_hrm_talasm_employee ON hrm_talent_assessments (employee_id, as_of_date DESC);

-- ── Development plans (SUBJECT-VISIBLE) ──────────────────────────────────────
-- The only succession table an employee may read about themselves.
--
-- ⚠ No plan_type, and no FK to a nomination. Both would leak the fact of a
-- nomination through a field the subject is entitled to see.
CREATE TABLE hrm_development_plans (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT NOT NULL UNIQUE
                    DEFAULT 'devplan_' || replace(gen_random_uuid()::text, '-', ''),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id     UUID NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    title           TEXT NOT NULL,
    objective       TEXT,
    target_date     DATE,
    status          TEXT NOT NULL DEFAULT 'draft',
    completed_at    TIMESTAMPTZ,

    created_by      UUID NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_devplan_status CHECK (
        status IN ('draft', 'active', 'completed', 'cancelled')
    ),
    CONSTRAINT chk_hrm_devplan_title CHECK (length(btrim(title)) > 0)
);

CREATE INDEX idx_hrm_devplan_org_id   ON hrm_development_plans (org_id);
CREATE INDEX idx_hrm_devplan_employee ON hrm_development_plans (employee_id, created_at DESC);

-- A plan is its actions. A title with no items is an intention.
CREATE TABLE hrm_development_plan_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT NOT NULL UNIQUE
                    DEFAULT 'devitem_' || replace(gen_random_uuid()::text, '-', ''),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    plan_id         UUID NOT NULL REFERENCES hrm_development_plans(id) ON DELETE CASCADE,

    description     TEXT NOT NULL,
    target_date     DATE,
    status          TEXT NOT NULL DEFAULT 'pending',
    completed_at    TIMESTAMPTZ,
    sort_order      INTEGER NOT NULL DEFAULT 0,

    created_by      UUID NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_devitem_status CHECK (
        status IN ('pending', 'in_progress', 'completed', 'cancelled')
    ),
    CONSTRAINT chk_hrm_devitem_desc CHECK (length(btrim(description)) > 0)
);

CREATE INDEX idx_hrm_devitem_org_id ON hrm_development_plan_items (org_id);
CREATE INDEX idx_hrm_devitem_plan   ON hrm_development_plan_items (plan_id, sort_order, created_at);

-- ── Succession candidates (CONFIDENTIAL) ─────────────────────────────────────
-- Declared last because it points at hrm_development_plans, and the FK
-- direction is the confidentiality guarantee: a nomination knows about a
-- plan, a plan knows nothing about a nomination.
CREATE TABLE hrm_succession_candidates (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id               TEXT NOT NULL UNIQUE
                            DEFAULT 'succand_' || replace(gen_random_uuid()::text, '-', ''),
    org_id                  UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    critical_position_id    UUID NOT NULL REFERENCES hrm_critical_positions(id) ON DELETE CASCADE,
    employee_id             UUID NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    readiness               TEXT NOT NULL DEFAULT 'ready_3_5_years',
    nomination_rationale    TEXT,
    development_plan_id     UUID REFERENCES hrm_development_plans(id) ON DELETE SET NULL,

    status                  TEXT NOT NULL DEFAULT 'active',
    withdrawn_at            TIMESTAMPTZ,
    withdrawn_reason        TEXT,

    nominated_by            UUID NOT NULL REFERENCES users(id),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_succand_readiness CHECK (
        readiness IN ('ready_now', 'ready_1_2_years', 'ready_3_5_years', 'emergency_cover')
    ),
    CONSTRAINT chk_hrm_succand_status CHECK (
        status IN ('active', 'withdrawn', 'placed')
    ),
    -- A withdrawal has to say when. Without the stamp a withdrawn nomination
    -- is indistinguishable from one that was never active.
    CONSTRAINT chk_hrm_succand_withdrawn CHECK (
        (status <> 'withdrawn') OR (withdrawn_at IS NOT NULL)
    )
);

-- One ACTIVE nomination per person per position. The same person may be
-- re-nominated after a withdrawal, and may be a candidate for several
-- positions at once — that is normal succession planning.
CREATE UNIQUE INDEX uq_hrm_succand_active
    ON hrm_succession_candidates (org_id, critical_position_id, employee_id)
    WHERE status = 'active';
CREATE INDEX idx_hrm_succand_org_id   ON hrm_succession_candidates (org_id);
CREATE INDEX idx_hrm_succand_position ON hrm_succession_candidates (critical_position_id, readiness);
CREATE INDEX idx_hrm_succand_employee ON hrm_succession_candidates (employee_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Reverse of creation order: hrm_succession_candidates first because it is
-- the only table holding an FK into another table in this migration.
DROP TABLE IF EXISTS hrm_succession_candidates;
DROP TABLE IF EXISTS hrm_development_plan_items;
DROP TABLE IF EXISTS hrm_development_plans;
DROP TABLE IF EXISTS hrm_talent_assessments;
DROP TABLE IF EXISTS hrm_critical_positions;

-- +goose StatementEnd
