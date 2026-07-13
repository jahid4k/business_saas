-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00021_hrm_salary_components
--
-- HRM salary engine tables (Group A1):
--   hrm_salary_components         — reusable earning/deduction building blocks
--   hrm_salary_structures         — named collection of components (e.g. "Grade A")
--   hrm_salary_structure_components — junction: which components belong to a structure
--   hrm_employee_salary_records   — append-only history of salary assignments
--
-- Design notes:
--   • calc_method drives how computed_value is derived:
--       fixed        → use fixed_value directly
--       pct_of_basic → fixed_value treated as percentage: BASIC * (fixed_value/100)
--       pct_of_gross → fixed_value as %: GROSS_SO_FAR * (fixed_value/100)
--       formula      → evaluate formula_expression via expr-lang/expr
--       manual       → value entered per payroll run (no formula)
--       slab         → progressive bracket calculation from slab_config JSONB
--   • formula_expression uses sandboxed expr-lang/expr — no arbitrary code.
--   • slab_config JSONB shape:
--       {"base_variable": "GROSS", "slabs": [{"up_to": 30000, "rate": 0.05}, ...]}
--   • hrm_employee_salary_records is append-only — updates create a new row.
--   • The row with the latest effective_date <= payroll_period is the active one.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_salary_components
-- ------------------------------------------------------------
CREATE TABLE hrm_salary_components (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id            TEXT        NOT NULL UNIQUE
                                         DEFAULT ('sc_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id               UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name                 TEXT        NOT NULL,
    description          TEXT,

    component_type       TEXT        NOT NULL
                                         CHECK (component_type IN ('earning', 'deduction', 'employer_contribution')),
    calc_method          TEXT        NOT NULL
                                         CHECK (calc_method IN ('fixed', 'pct_of_basic', 'pct_of_gross', 'formula', 'manual', 'slab')),

    -- Used by: fixed, pct_of_basic, pct_of_gross
    fixed_value          NUMERIC(15,4) NOT NULL DEFAULT 0,

    -- Used by: formula (expr-lang expression evaluated at payroll time)
    formula_expression   TEXT,
    formula_variables    TEXT[],      -- documentation: which env vars this formula uses

    -- Used by: slab (JSONB progressive bracket config)
    slab_config          JSONB,

    is_taxable           BOOLEAN     NOT NULL DEFAULT FALSE,
    display_order        INTEGER     NOT NULL DEFAULT 0,
    is_active            BOOLEAN     NOT NULL DEFAULT TRUE,

    created_by           UUID        NOT NULL REFERENCES users(id),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_sc_org_id ON hrm_salary_components (org_id);
CREATE UNIQUE INDEX idx_hrm_sc_org_name
    ON hrm_salary_components (org_id, LOWER(name))
    WHERE is_active = TRUE;

COMMENT ON TABLE  hrm_salary_components IS 'Reusable earning/deduction building blocks for payroll';
COMMENT ON COLUMN hrm_salary_components.calc_method        IS 'fixed|pct_of_basic|pct_of_gross|formula|manual|slab';
COMMENT ON COLUMN hrm_salary_components.formula_expression IS 'expr-lang expression; env: BASIC, GROSS, TENURE_YEARS, etc.';
COMMENT ON COLUMN hrm_salary_components.slab_config        IS '{"base_variable":"GROSS","slabs":[{"up_to":30000,"rate":0.05},{"up_to":null,"rate":0.10}]}';

-- ------------------------------------------------------------
-- hrm_salary_structures
-- ------------------------------------------------------------
CREATE TABLE hrm_salary_structures (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id     TEXT        NOT NULL UNIQUE
                                  DEFAULT ('ss_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id        UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name          TEXT        NOT NULL,
    description   TEXT,
    grade_label   TEXT,        -- optional: "Grade A", "Senior", "Executive"
    is_active     BOOLEAN     NOT NULL DEFAULT TRUE,

    created_by    UUID        NOT NULL REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_ss_org_id ON hrm_salary_structures (org_id);
CREATE UNIQUE INDEX idx_hrm_ss_org_name
    ON hrm_salary_structures (org_id, LOWER(name))
    WHERE is_active = TRUE;

COMMENT ON TABLE hrm_salary_structures IS 'Named grouping of salary components (e.g. Engineering Grade A)';

-- ------------------------------------------------------------
-- hrm_salary_structure_components  (junction)
-- ------------------------------------------------------------
CREATE TABLE hrm_salary_structure_components (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    structure_id    UUID        NOT NULL REFERENCES hrm_salary_structures(id) ON DELETE CASCADE,
    component_id    UUID        NOT NULL REFERENCES hrm_salary_components(id) ON DELETE RESTRICT,

    -- Per-structure override: if set, use this value instead of component.fixed_value
    override_value  NUMERIC(15,4),

    display_order   INTEGER     NOT NULL DEFAULT 0,

    CONSTRAINT uq_hrm_ssc UNIQUE (structure_id, component_id)
);

CREATE INDEX idx_hrm_ssc_structure_id ON hrm_salary_structure_components (structure_id);
CREATE INDEX idx_hrm_ssc_component_id ON hrm_salary_structure_components (component_id);

COMMENT ON TABLE  hrm_salary_structure_components IS 'Junction: which components belong to each salary structure';
COMMENT ON COLUMN hrm_salary_structure_components.override_value IS 'Per-structure value override; NULL means use component.fixed_value';

-- ------------------------------------------------------------
-- hrm_employee_salary_records  (append-only history)
-- ------------------------------------------------------------
CREATE TABLE hrm_employee_salary_records (
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id        TEXT          NOT NULL UNIQUE
                                       DEFAULT ('esr_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id           UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id      UUID          NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,
    structure_id     UUID          REFERENCES hrm_salary_structures(id) ON DELETE SET NULL,

    basic_pay        NUMERIC(15,2) NOT NULL CHECK (basic_pay >= 0),
    effective_date   DATE          NOT NULL,

    -- Reason for this record (joining, promotion, annual revision, etc.)
    change_reason    TEXT          NOT NULL
                                       CHECK (change_reason IN
                                           ('joining', 'promotion', 'annual_revision',
                                            'transfer', 'correction', 'other')),
    change_notes     TEXT,

    created_by       UUID          NOT NULL REFERENCES users(id),
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW()
    -- No updated_at — append-only; create new row instead of updating
);

CREATE INDEX idx_hrm_esr_org_id      ON hrm_employee_salary_records (org_id);
CREATE INDEX idx_hrm_esr_employee_id ON hrm_employee_salary_records (employee_id);
CREATE INDEX idx_hrm_esr_effective   ON hrm_employee_salary_records (employee_id, effective_date DESC);

COMMENT ON TABLE  hrm_employee_salary_records IS 'Append-only salary history; query MAX(effective_date) <= period for active record';
COMMENT ON COLUMN hrm_employee_salary_records.basic_pay IS 'Base salary; component formulas use this as BASIC variable';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_employee_salary_records;
DROP TABLE IF EXISTS hrm_salary_structure_components;
DROP TABLE IF EXISTS hrm_salary_structures;
DROP TABLE IF EXISTS hrm_salary_components;

-- +goose StatementEnd
