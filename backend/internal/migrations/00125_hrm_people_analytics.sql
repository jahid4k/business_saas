-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00125_hrm_people_analytics
--
-- Phase 10C: people analytics. Three tables:
--
--   hrm_metric_definitions   — what a metric MEANS, as data
--   hrm_headcount_snapshots  — nightly point-in-time facts
--   hrm_attrition_facts      — one immutable row per exit
--
-- ⚠ NIGHTLY SNAPSHOTS AND FACT TABLES, NEVER LIVE AGGREGATION OVER OLTP.
--
-- This is the plan's rule and the reason internal/hrm/reports' COUNT(*)
-- queries are NOT extended into analytics. Those three endpoints are cheap,
-- already shipped, and answer "how many employees are there right now" —
-- which is an OLTP question. Analytics asks "what was headcount on the 1st of
-- each of the last 24 months, split by department" and every honest answer to
-- that is a scan of live tables that grows with the business. Worse, the
-- answer would change retroactively: an employee whose department is
-- corrected today would silently rewrite last March.
--
-- So the fact tables are the ONLY thing the analytics read path touches. The
-- nightly job is the only thing that reads OLTP. An integration test mutates
-- OLTP and asserts the metric does not move until the job has run.
--
-- ⚠ hrm_metric_definitions IS NOT A FORMULA ENGINE, AND MUST NOT BECOME ONE.
--
-- The plan calls it non-optional: a metric's formula, grain and filters must
-- be data so two consumers cannot compute "attrition" two ways. But this
-- codebase already has one interpreted-expression defect open —
-- learning's evalFormula evaluates user formulas in float64 — and building a
-- second interpreter to fix a definitional problem would be a worse trade
-- than the problem.
--
-- So the split is: `computation` names a Go implementation from a CHECKed
-- vocabulary, and every PARAMETER of that computation is data — which
-- termination types count as attrition, whether probation exits are
-- included, the period basis, the suppression threshold. `formula_statement`
-- is the human-readable definition that must agree with the named
-- computation; it is documentation with a constraint on it, never something
-- that gets parsed. Two consumers therefore cannot disagree about what
-- attrition means, and nothing evaluates a string.
--
-- ⚠ PREDICTIVE SCORING IS DELIBERATELY EXCLUDED (plan). There is no
-- risk_score, no propensity, no forecast column anywhere below, for the same
-- reason 10B has none: a number nobody can explain gets acted on by people
-- who cannot say what it means.
--
-- ⚠ DEI IS AGGREGATE-ONLY AND THRESHOLD-GATED, AND THE THRESHOLD BINDS
-- view_all HOLDERS TOO.
--
-- suppression_threshold lives on the metric definition rather than in code
-- because the right number is a legal and cultural judgement that differs by
-- country. The rule it drives is implemented in Go (see metrics.go): a group
-- below the threshold reports only that it is suppressed, at least TWO groups
-- are always suppressed when any is, and the TOTAL is withheld entirely
-- whenever suppression occurs — because a total plus every disclosed group is
-- a subtraction away from the one that was hidden. That is 5C's suppression
-- rule, and no permission lifts it.
-- ============================================================

-- ── Metric definitions ───────────────────────────────────────────────────────
CREATE TABLE hrm_metric_definitions (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id               TEXT NOT NULL UNIQUE
                            DEFAULT 'metric_' || replace(gen_random_uuid()::text, '-', ''),
    org_id                  UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    metric_key              TEXT NOT NULL,
    name                    TEXT NOT NULL,
    description             TEXT,

    -- Names a Go implementation. NOT parsed, NOT evaluated.
    computation             TEXT NOT NULL,
    -- The same definition in words, so a reader can check that the named
    -- computation is the one they think they are looking at.
    formula_statement       TEXT NOT NULL,
    grain                   TEXT NOT NULL DEFAULT 'org',

    -- Parameters. These are the part that is genuinely data.
    attrition_types         TEXT[],
    include_probation_exits BOOLEAN NOT NULL DEFAULT TRUE,
    suppression_threshold   INTEGER NOT NULL DEFAULT 5,

    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_by              UUID NOT NULL REFERENCES users(id),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_metric_computation CHECK (
        computation IN ('headcount', 'attrition_rate', 'first_year_attrition',
                        'cohort_retention', 'tenure_distribution',
                        'dei_distribution', 'compensation_distribution')
    ),
    CONSTRAINT chk_hrm_metric_grain CHECK (
        grain IN ('org', 'department', 'legal_entity')
    ),
    -- A threshold of 0 or 1 discloses an individual and is never a valid
    -- suppression setting, whatever an administrator types.
    CONSTRAINT chk_hrm_metric_threshold CHECK (suppression_threshold >= 2),
    CONSTRAINT chk_hrm_metric_statement CHECK (length(btrim(formula_statement)) > 0)
);

CREATE UNIQUE INDEX uq_hrm_metric_org_key ON hrm_metric_definitions (org_id, metric_key);
CREATE INDEX idx_hrm_metric_org_id ON hrm_metric_definitions (org_id) WHERE is_active;

-- ── Headcount snapshots ──────────────────────────────────────────────────────
-- One row per (org, date, dimension, dimension member). The org total is
-- dimension='org' with a NULL dimension_id.
CREATE TABLE hrm_headcount_snapshots (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id           TEXT NOT NULL UNIQUE
                        DEFAULT 'hcsnap_' || replace(gen_random_uuid()::text, '-', ''),
    org_id              UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    snapshot_date       DATE NOT NULL,

    dimension           TEXT NOT NULL DEFAULT 'org',
    dimension_id        UUID,

    headcount           INTEGER NOT NULL DEFAULT 0,
    joiners             INTEGER NOT NULL DEFAULT 0,
    leavers             INTEGER NOT NULL DEFAULT 0,
    voluntary_leavers   INTEGER NOT NULL DEFAULT 0,
    involuntary_leavers INTEGER NOT NULL DEFAULT 0,
    regretted_leavers   INTEGER NOT NULL DEFAULT 0,
    avg_tenure_days     INTEGER NOT NULL DEFAULT 0,

    -- Compensation distribution, separately permissioned on read.
    --
    -- ⚠ NULL whenever headcount is below the suppression threshold: a median
    -- over two people is two people's salaries with a statistic drawn on top.
    -- The nightly job leaves these NULL rather than the read path blanking
    -- them, so a small group's pay is never written down in the first place.
    comp_p25            NUMERIC(15,2),
    comp_median         NUMERIC(15,2),
    comp_p75            NUMERIC(15,2),
    comp_currency       CHAR(3),

    computed_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_hcsnap_dimension CHECK (
        dimension IN ('org', 'department', 'legal_entity')
    ),
    -- The org total has no member id; a member dimension must name one.
    CONSTRAINT chk_hrm_hcsnap_dim_id CHECK (
        (dimension = 'org' AND dimension_id IS NULL) OR
        (dimension <> 'org' AND dimension_id IS NOT NULL)
    ),
    CONSTRAINT chk_hrm_hcsnap_nonneg CHECK (
        headcount >= 0 AND joiners >= 0 AND leavers >= 0 AND
        voluntary_leavers >= 0 AND involuntary_leavers >= 0 AND regretted_leavers >= 0
    )
);

-- COALESCE rather than a plain unique index: NULL never equals NULL, so a
-- three-column unique index would happily allow two org-total rows for the
-- same day and the job would double-count on its second run.
CREATE UNIQUE INDEX uq_hrm_hcsnap_day
    ON hrm_headcount_snapshots (org_id, snapshot_date, dimension,
        COALESCE(dimension_id, '00000000-0000-0000-0000-000000000000'::uuid));
CREATE INDEX idx_hrm_hcsnap_org_date ON hrm_headcount_snapshots (org_id, snapshot_date DESC);

-- ── Attrition facts ──────────────────────────────────────────────────────────
-- One row per exit, denormalized at fact-build time.
--
-- The dimensions are SNAPSHOT, not FK-resolved on read, because an employee
-- who transfers department after leaving — a correction, a reorganisation
-- that renumbers departments — would otherwise rewrite history.
CREATE TABLE hrm_attrition_facts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id           TEXT NOT NULL UNIQUE
                        DEFAULT 'attfact_' || replace(gen_random_uuid()::text, '-', ''),
    org_id              UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id         UUID NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,
    exit_id             UUID REFERENCES hrm_exits(id) ON DELETE SET NULL,

    exit_date           DATE NOT NULL,
    hire_date           DATE NOT NULL,
    cohort_month        DATE NOT NULL,
    tenure_days         INTEGER NOT NULL,
    is_first_year       BOOLEAN NOT NULL,

    source_type         TEXT NOT NULL,
    termination_type    TEXT,
    is_voluntary        BOOLEAN NOT NULL,

    -- ⚠ NULLABLE ON PURPOSE. is_regretted comes from Phase 9's
    -- hrm_rehire_eligibility.status, which 9A gave its first reader:
    -- 'eligible' is regretted, 'not_eligible' is not, and 'conditional' or a
    -- missing row is genuinely UNKNOWN. Defaulting unknown to false would
    -- report every un-reviewed exit as non-regretted and quietly flatter the
    -- number this metric exists to expose.
    is_regretted        BOOLEAN,

    department_id       UUID,
    position_id         UUID,
    legal_entity_id     UUID,
    gender              TEXT,

    computed_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_attfact_source CHECK (source_type IN ('resignation', 'termination')),
    CONSTRAINT chk_hrm_attfact_tenure CHECK (tenure_days >= 0),
    CONSTRAINT chk_hrm_attfact_gender CHECK (
        gender IS NULL OR gender IN ('male', 'female', 'other', 'prefer_not_to_say')
    )
);

-- An employee can be rehired and leave again, so the key is the exit date
-- rather than the employee.
CREATE UNIQUE INDEX uq_hrm_attfact_employee_exit
    ON hrm_attrition_facts (org_id, employee_id, exit_date);
CREATE INDEX idx_hrm_attfact_org_date   ON hrm_attrition_facts (org_id, exit_date DESC);
CREATE INDEX idx_hrm_attfact_cohort     ON hrm_attrition_facts (org_id, cohort_month);
CREATE INDEX idx_hrm_attfact_department ON hrm_attrition_facts (org_id, department_id)
    WHERE department_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_attrition_facts;
DROP TABLE IF EXISTS hrm_headcount_snapshots;
DROP TABLE IF EXISTS hrm_metric_definitions;

-- +goose StatementEnd
