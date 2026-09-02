-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00102_hrm_statutory
--
-- Phase 7D, first half: statutory compliance. Two tables:
--
--   hrm_statutory_rules   — a stable definition (what kind of deduction or
--                           contribution this is, which side pays it)
--   hrm_statutory_slabs   — the effective-dated bracket table a rule reads
--
-- Design notes:
--
--   • The build plan's own scope note reads "no country implementation
--     yet", but shipping a bare interface with zero implementations is
--     exactly the speculative primitive rule 1 forbids. The resolution,
--     built in Go (internal/hrm/statutory), not here: a country-pluggable
--     `Provider` interface + registry, with ONE real, DATA-DRIVEN provider
--     that reads these tables and evaluates them via the already-tested
--     `payslips.ComputeSlab` — the exact function hrm_salary_components'
--     slab calc_method already uses (00023). That gives the interface a
--     real consumer from day one; country-specific Go providers
--     (proration rules, eligibility thresholds — the things a slab table
--     cannot express) register alongside it later without a schema change.
--
--   • hrm_statutory_rules is the STABLE identity (name, rule_type,
--     base_variable, is_employer_contribution, country_code); slabs are
--     effective-dated data hanging off it. Same split as 00098's
--     hrm_merit_matrix_cells vs. the rating level it keys on — a rule does
--     not change identity when its bracket table is repriced.
--
--   • is_employer_contribution is its own boolean, not a derived value —
--     the hrm_bonuses / hrm_payslip_lines precedent (00096/00098). An org
--     wanting BOTH an employee-paid and an employer-paid side of the same
--     scheme (social security is the standard example) creates TWO rule
--     rows sharing the same rule_type, one flagged each way — simpler than
--     splitting a single row's slab table into two columns per bracket.
--
--   • base_variable reuses hrm_salary_components.slab_config's own
--     vocabulary ('GROSS', 'BASIC') plus a new 'TAXABLE_GROSS' — the sum of
--     only the salary components flagged is_taxable (00023). Statutory
--     rules are commonly computed against taxable income specifically, not
--     raw gross, which is why this base did not already exist in payroll:
--     nothing needed it until now.
--
--   • country_code is stored but NOT used to filter which employees a rule
--     applies to — hrm_employees carries no country field yet (multi-country
--     is explicit Phase 11 scope, 00070's hrm_legal_entities scaffold has
--     zero business logic). Today it is descriptive metadata on the rule;
--     every active rule for an org applies to every payroll-eligible
--     employee in that org. Filtering by employee country is Phase 11's job,
--     not invented here ahead of the data that would make it meaningful.
--
--   • No new column on hrm_salary_components. is_taxable already exists
--     (00023) — the statutory base was already modellable.
--
-- What must NEVER be added (the 00076 CHECK x ON DELETE SET NULL trap):
-- nothing here is ON DELETE SET NULL onto a column a CHECK could pair with
-- — hrm_statutory_slabs is ON DELETE CASCADE from its rule, not SET NULL,
-- specifically to avoid recreating that trap.
-- ============================================================

CREATE TABLE hrm_statutory_rules (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id                TEXT        NOT NULL UNIQUE
                                             DEFAULT ('sxr_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                   UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name                     TEXT        NOT NULL,
    country_code             CHAR(2)     NOT NULL,
    rule_type                TEXT        NOT NULL
                                             CHECK (rule_type IN ('income_tax', 'social_security', 'provident_fund', 'other')),
    base_variable            TEXT        NOT NULL DEFAULT 'TAXABLE_GROSS'
                                             CHECK (base_variable IN ('GROSS', 'BASIC', 'TAXABLE_GROSS')),
    is_employer_contribution BOOLEAN     NOT NULL DEFAULT FALSE,
    is_active                BOOLEAN     NOT NULL DEFAULT TRUE,

    created_by               UUID        NOT NULL REFERENCES users(id),
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_sxr_org_id ON hrm_statutory_rules (org_id, is_active);

COMMENT ON TABLE  hrm_statutory_rules IS 'Stable statutory rule identity; hrm_statutory_slabs holds the effective-dated bracket data it evaluates';
COMMENT ON COLUMN hrm_statutory_rules.country_code IS 'Descriptive only today — no employee-level country field exists yet (Phase 11). Every active rule applies org-wide.';

CREATE TABLE hrm_statutory_slabs (
    id                UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id         TEXT          NOT NULL UNIQUE
                                        DEFAULT ('sxs_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    rule_id           UUID          NOT NULL REFERENCES hrm_statutory_rules(id) ON DELETE CASCADE,

    -- Slab bracket, mirroring hrm_salary_components.slab_config's shape
    -- (00023) but as real rows — the same "slabs move from code/JSONB into
    -- effective-dated rows" instruction 00098 already applied to the merit
    -- matrix.
    up_to             NUMERIC(15,2),  -- NULL = no upper bound (last bracket)
    rate_pct          NUMERIC(6,3)   NOT NULL CHECK (rate_pct >= 0),
    effective_date    DATE          NOT NULL,

    created_by         UUID          NOT NULL REFERENCES users(id),
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_sxs_rule_id ON hrm_statutory_slabs (rule_id, effective_date DESC);

COMMENT ON TABLE hrm_statutory_slabs IS 'Effective-dated progressive bracket table for one statutory rule, evaluated via payslips.ComputeSlab — the exact function hrm_salary_components slab components already use';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_statutory_slabs;
DROP TABLE IF EXISTS hrm_statutory_rules;

-- +goose StatementEnd
