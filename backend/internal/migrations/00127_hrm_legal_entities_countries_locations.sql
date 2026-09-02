-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00127_hrm_legal_entities_countries_locations
--
-- Phase 11A: the legal-entity layer becomes real. One table extended, two
-- created, one column added:
--
--   hrm_legal_entities   — EXTENDED with country, currency, registration
--   hrm_country_configs  — per-country statutory and payroll defaults
--   hrm_locations        — work sites belonging to an entity
--   hrm_employees.location_id — where somebody actually works
--
-- ⚠ legal_entity_id IS ALREADY AN FK ON 38 TABLES, ALL NULLABLE, AND IT
-- STAYS THAT WAY.
--
-- Phase 0.4 planted the column across the schema before there was anything to
-- put in it, which is why this phase writes logic rather than doing schema
-- surgery. Nothing here backfills those 38 columns to a default entity, and
-- nothing here makes any of them NOT NULL. A single-entity organization — or
-- one with no entities at all, which is every organization in this database
-- today — must keep working completely untouched, and a backfill would
-- rewrite 38 tables to record a fact nobody has asserted.
--
-- Resolution is therefore a FALLBACK CHAIN, not a stored value:
--
--     entity-specific → the org's default entity → the organization itself
--
-- the same shape as hrm_per_diem_rates' country-specific → org-wide lookup
-- (`ORDER BY (country_code IS NULL)`) and platform_sla_policies. A row with a
-- NULL legal_entity_id is not missing data; it means "whatever the
-- organization's default is", and that answer stays correct when the default
-- changes.
--
-- ⚠ country_code AND base_currency ARE NULLABLE ON PURPOSE. Entities created
-- before this migration have neither, and there is no honest value to write:
-- guessing a country for somebody's subsidiary is worse than resolving
-- organizations.country at read time, because a guess looks like a fact.
--
-- ⚠ created_by IS NULLABLE for the same reason. Rows predating the column
-- have no known author, and attributing them to whoever runs the migration
-- would be a lie recorded in an audit field.
--
-- ⚠ THE ONE-DEFAULT-PER-ORG CONSTRAINT ALREADY EXISTS and is NOT recreated
-- here: idx_hrm_legal_entities_org_default is a partial unique index on
-- (org_id) WHERE is_default, planted in 0.4. That constraint is what makes
-- step two of the resolution chain single-valued, so it is load-bearing for
-- this phase; it was verified present rather than assumed.
-- ============================================================

-- ── hrm_legal_entities: extend ───────────────────────────────────────────────

ALTER TABLE hrm_legal_entities
    ADD COLUMN public_id           TEXT,
    ADD COLUMN country_code        CHAR(2),
    ADD COLUMN base_currency       CHAR(3),
    ADD COLUMN registration_number TEXT,
    ADD COLUMN tax_identifier      TEXT,
    ADD COLUMN registered_address  TEXT,
    ADD COLUMN timezone            TEXT,
    ADD COLUMN is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN created_by          UUID REFERENCES users(id);

-- public_id is backfilled explicitly rather than relying on a volatile
-- column default. PostgreSQL does evaluate a volatile default per row on ADD
-- COLUMN, but depending on that is a subtle thing to depend on when the
-- alternative is one UPDATE that plainly does what it says.
UPDATE hrm_legal_entities
   SET public_id = 'lentity_' || replace(gen_random_uuid()::text, '-', '')
 WHERE public_id IS NULL;

ALTER TABLE hrm_legal_entities
    ALTER COLUMN public_id SET NOT NULL,
    ALTER COLUMN public_id SET DEFAULT 'lentity_' || replace(gen_random_uuid()::text, '-', ''),
    ADD CONSTRAINT hrm_legal_entities_public_id_key UNIQUE (public_id);

ALTER TABLE hrm_legal_entities
    ADD CONSTRAINT chk_hrm_lentity_country CHECK (
        country_code IS NULL OR country_code ~ '^[A-Z]{2}$'
    ),
    ADD CONSTRAINT chk_hrm_lentity_currency CHECK (
        base_currency IS NULL OR base_currency ~ '^[A-Z]{3}$'
    );

CREATE INDEX idx_hrm_lentity_country ON hrm_legal_entities (org_id, country_code)
    WHERE country_code IS NOT NULL;

-- ── hrm_country_configs ──────────────────────────────────────────────────────
-- Per-country defaults an entity in that country inherits.
--
-- ⚠ These are DEFAULTS, not enforcement. Nothing in payroll reads this table
-- and refuses to run; a config supplies a value where a caller has not
-- specified one, which is why every column below is nullable. A country
-- config that hard-required a notice period would break the first
-- organization whose contracts differ from the statutory minimum, which is
-- most of them.
CREATE TABLE hrm_country_configs (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id                   TEXT NOT NULL UNIQUE
                                DEFAULT 'ccfg_' || replace(gen_random_uuid()::text, '-', ''),
    org_id                      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    country_code                CHAR(2) NOT NULL,
    country_name                TEXT,

    default_currency            CHAR(3),
    payroll_cycle               TEXT,
    pay_day_of_month            INTEGER,
    fiscal_year_start_month     INTEGER,

    standard_work_days_per_week NUMERIC(3,1),
    standard_hours_per_day      NUMERIC(4,2),
    overtime_multiplier         NUMERIC(4,2),

    annual_leave_days           INTEGER,
    notice_period_days          INTEGER,
    probation_days              INTEGER,

    -- Feeds 9B's gratuity computation, which currently takes these as
    -- arguments from the caller.
    gratuity_eligible_years     NUMERIC(4,2),
    gratuity_days_per_year      NUMERIC(4,1),

    is_active                   BOOLEAN NOT NULL DEFAULT TRUE,
    created_by                  UUID NOT NULL REFERENCES users(id),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_ccfg_country  CHECK (country_code ~ '^[A-Z]{2}$'),
    CONSTRAINT chk_hrm_ccfg_currency CHECK (default_currency IS NULL OR default_currency ~ '^[A-Z]{3}$'),
    CONSTRAINT chk_hrm_ccfg_cycle CHECK (
        payroll_cycle IS NULL OR
        payroll_cycle IN ('monthly', 'semi_monthly', 'biweekly', 'weekly')
    ),
    CONSTRAINT chk_hrm_ccfg_payday CHECK (
        pay_day_of_month IS NULL OR pay_day_of_month BETWEEN 1 AND 31
    ),
    CONSTRAINT chk_hrm_ccfg_fiscal CHECK (
        fiscal_year_start_month IS NULL OR fiscal_year_start_month BETWEEN 1 AND 12
    ),
    CONSTRAINT chk_hrm_ccfg_nonneg CHECK (
        COALESCE(annual_leave_days, 0) >= 0 AND
        COALESCE(notice_period_days, 0) >= 0 AND
        COALESCE(probation_days, 0) >= 0 AND
        COALESCE(standard_work_days_per_week, 0) >= 0 AND
        COALESCE(standard_hours_per_day, 0) >= 0 AND
        COALESCE(overtime_multiplier, 1) >= 0 AND
        COALESCE(gratuity_eligible_years, 0) >= 0 AND
        COALESCE(gratuity_days_per_year, 0) >= 0
    )
);

CREATE UNIQUE INDEX uq_hrm_ccfg_org_country ON hrm_country_configs (org_id, country_code);
CREATE INDEX idx_hrm_ccfg_org_id ON hrm_country_configs (org_id) WHERE is_active;

-- ── hrm_locations ────────────────────────────────────────────────────────────
-- A work site. legal_entity_id is NULLABLE for the same reason it is nullable
-- everywhere else: a site can be recorded before anybody sets up entities,
-- and forcing one would make this table unusable for the single-entity orgs
-- that are the majority.
CREATE TABLE hrm_locations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT NOT NULL UNIQUE
                    DEFAULT 'loc_' || replace(gen_random_uuid()::text, '-', ''),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    legal_entity_id UUID REFERENCES hrm_legal_entities(id) ON DELETE SET NULL,

    name            TEXT NOT NULL,
    code            TEXT,
    address_line1   TEXT,
    address_line2   TEXT,
    city            TEXT,
    state           TEXT,
    postal_code     TEXT,
    country_code    CHAR(2),
    timezone        TEXT,

    is_headquarters BOOLEAN NOT NULL DEFAULT FALSE,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,

    created_by      UUID NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_loc_name    CHECK (length(btrim(name)) > 0),
    CONSTRAINT chk_hrm_loc_country CHECK (country_code IS NULL OR country_code ~ '^[A-Z]{2}$')
);

-- Codes are how a location is referenced in an import or a payroll file, so
-- they must be unique among ACTIVE sites. Retiring a site frees its code.
CREATE UNIQUE INDEX uq_hrm_loc_active_code
    ON hrm_locations (org_id, lower(code)) WHERE is_active AND code IS NOT NULL;
-- One headquarters per org, the hrm_legal_entities.is_default shape.
CREATE UNIQUE INDEX uq_hrm_loc_headquarters
    ON hrm_locations (org_id) WHERE is_headquarters AND is_active;
CREATE INDEX idx_hrm_loc_org_id ON hrm_locations (org_id);
CREATE INDEX idx_hrm_loc_entity ON hrm_locations (legal_entity_id)
    WHERE legal_entity_id IS NOT NULL;

-- ── hrm_employees.location_id ────────────────────────────────────────────────
-- Without this the locations table is a list nothing references. NULLABLE,
-- like every other structural FK on hrm_employees (department_id,
-- position_id, manager_id, legal_entity_id all are), so no existing row and
-- no existing insert path changes.
ALTER TABLE hrm_employees
    ADD COLUMN location_id UUID REFERENCES hrm_locations(id) ON DELETE SET NULL;

CREATE INDEX idx_hrm_emp_location_id ON hrm_employees (location_id)
    WHERE location_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- ⚠ Reverse of creation order, and NOT the mirror of the ALTER list.
-- hrm_employees.location_id must go before hrm_locations, because the FK
-- points that way; dropping the table first would fail.
DROP INDEX IF EXISTS idx_hrm_emp_location_id;
ALTER TABLE hrm_employees DROP COLUMN IF EXISTS location_id;

DROP TABLE IF EXISTS hrm_locations;
DROP TABLE IF EXISTS hrm_country_configs;

-- hrm_legal_entities is RESTORED, not dropped: it existed before this
-- migration with rows in it, and dropping it would destroy data 0.4 created
-- and 38 foreign keys depend on.
DROP INDEX IF EXISTS idx_hrm_lentity_country;
ALTER TABLE hrm_legal_entities
    DROP CONSTRAINT IF EXISTS chk_hrm_lentity_country,
    DROP CONSTRAINT IF EXISTS chk_hrm_lentity_currency,
    DROP CONSTRAINT IF EXISTS hrm_legal_entities_public_id_key;

ALTER TABLE hrm_legal_entities
    DROP COLUMN IF EXISTS public_id,
    DROP COLUMN IF EXISTS country_code,
    DROP COLUMN IF EXISTS base_currency,
    DROP COLUMN IF EXISTS registration_number,
    DROP COLUMN IF EXISTS tax_identifier,
    DROP COLUMN IF EXISTS registered_address,
    DROP COLUMN IF EXISTS timezone,
    DROP COLUMN IF EXISTS is_active,
    DROP COLUMN IF EXISTS created_by;

-- +goose StatementEnd
