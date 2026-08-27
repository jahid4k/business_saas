-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00106_hrm_assets
--
-- Phase 8A: asset management. Seven tables:
--
--   hrm_asset_categories        — catalog: what kinds of asset exist
--   hrm_assets                  — the physical instances
--   hrm_asset_assignments       — who held what, when (append-shaped history)
--   hrm_asset_maintenance_logs  — service/repair history per asset
--   hrm_asset_requests          — an employee asking for an asset (approval-gated)
--   hrm_software_licenses       — licences, a DIFFERENT shape from physical assets
--   hrm_license_seat_assignments— seat occupancy within a licence
--
-- Design notes:
--
--   • THE CURRENT HOLDER IS NEVER A STORED COLUMN. The build plan is emphatic
--     about this and it is the 00076 computed-not-stored rule: an asset's
--     current holder is the hrm_asset_assignments row with returned_at IS
--     NULL, derived on every read. A denormalized hrm_assets.current_holder_id
--     would be a second source of truth that silently drifts the first time a
--     return is recorded without updating it. The partial unique index
--     uq_hrm_asgn_active is what makes the derived query single-valued.
--
--     Identically: a licence's seats_used is COUNT(*) over unreleased seat
--     assignments, never a counter column. Same reasoning, same trap.
--
--     An integration test introspects information_schema to assert neither
--     column exists — the 6A precedent, where course completion percentage
--     got the same treatment.
--
--   • Depreciation is a BOOK-VALUE STUB and is likewise computed, not stored:
--     straight-line from purchase_cost over useful_life_months. Real
--     fixed-asset accounting (revaluation, disposal gain/loss, tax schedules)
--     belongs to the Accounting module, and storing a book_value column here
--     would quietly claim this is that.
--
--   • Software licences are a SEPARATE SHAPE, not a category of hrm_assets.
--     A physical asset has exactly one holder at a time and a serial number;
--     a licence has N seats, a renewal date, and a per-seat cost. Forcing them
--     into one table means every physical-asset query carries a seats_total
--     column it never reads, and every licence query carries a serial_number
--     it cannot populate. The build plan names this distinction directly.
--
--   • hrm_asset_categories.requires_return drives exit clearance (Phase 9) and
--     payroll recovery: a laptop must come back, a branded t-shirt need not.
--     Stored on the CATEGORY, not the instance — it is a property of the kind
--     of thing, and per-instance overrides have no requester today.
--
--   • Handover sign-off reuses hrm_acknowledgements via a widened
--     acknowledgeable_type ('asset_handover'). Third such widening — 5B added
--     'appraisal' (00086), 6B added 'course_completion' (00094).
--
-- What must NEVER be added (the 00076 CHECK x ON DELETE SET NULL trap):
-- Postgres re-evaluates CHECKs on UPDATE and ON DELETE SET NULL *is* an
-- UPDATE. hrm_asset_assignments.returned_by and hrm_assets.category_id are
-- both ON DELETE SET NULL, so:
--   • CHECK (returned_at IS NULL OR returned_by IS NOT NULL) would make
--     DELETE FROM users fail 23514 for any org holding a returned assignment.
--   • CHECK (category_id IS NOT NULL OR ...) would do the same for categories.
-- The service validates those pairings instead.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_asset_categories
-- ------------------------------------------------------------
CREATE TABLE hrm_asset_categories (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id           TEXT        NOT NULL UNIQUE
                                        DEFAULT ('acat_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id              UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name                TEXT        NOT NULL,
    description         TEXT,
    -- Drives exit clearance (Phase 9) and payroll recovery.
    requires_return     BOOLEAN     NOT NULL DEFAULT TRUE,
    -- Depreciation inputs. NULL useful_life_months means "not depreciated" —
    -- BookValue() returns purchase_cost unchanged rather than dividing by zero.
    useful_life_months  INTEGER     CHECK (useful_life_months IS NULL OR useful_life_months > 0),

    is_active           BOOLEAN     NOT NULL DEFAULT TRUE,
    created_by          UUID        NOT NULL REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX        idx_hrm_acat_org_id ON hrm_asset_categories (org_id, is_active);
CREATE UNIQUE INDEX uq_hrm_acat_org_name ON hrm_asset_categories (org_id, LOWER(name));

COMMENT ON COLUMN hrm_asset_categories.requires_return IS 'A property of the KIND of asset, not the instance — drives exit clearance and payroll recovery';

-- ------------------------------------------------------------
-- hrm_assets
-- ------------------------------------------------------------
CREATE TABLE hrm_assets (
    id             UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id      TEXT          NOT NULL UNIQUE
                                     DEFAULT ('ast_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id         UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    category_id    UUID          REFERENCES hrm_asset_categories(id) ON DELETE SET NULL,

    name           TEXT          NOT NULL,
    asset_tag      TEXT,
    serial_number  TEXT,
    purchase_date  DATE,
    purchase_cost  NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (purchase_cost >= 0),
    currency       CHAR(3)       NOT NULL DEFAULT 'USD',

    status         TEXT          NOT NULL DEFAULT 'available'
                                     CHECK (status IN ('available', 'assigned', 'in_maintenance', 'retired', 'lost')),
    notes          TEXT,

    created_by     UUID          NOT NULL REFERENCES users(id),
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_ast_org_id   ON hrm_assets (org_id, status);
CREATE INDEX idx_hrm_ast_category ON hrm_assets (category_id) WHERE category_id IS NOT NULL;
CREATE UNIQUE INDEX uq_hrm_ast_org_tag ON hrm_assets (org_id, asset_tag) WHERE asset_tag IS NOT NULL;

COMMENT ON TABLE hrm_assets IS 'Physical asset instances. There is deliberately NO current_holder column — the holder is derived from hrm_asset_assignments WHERE returned_at IS NULL. See migration header.';

-- ------------------------------------------------------------
-- hrm_asset_assignments
-- ------------------------------------------------------------
CREATE TABLE hrm_asset_assignments (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT        NOT NULL UNIQUE
                                    DEFAULT ('asgn_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id          UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    asset_id        UUID        NOT NULL REFERENCES hrm_assets(id) ON DELETE CASCADE,
    employee_id     UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    assigned_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_by     UUID        NOT NULL REFERENCES users(id),
    condition_out   TEXT        CHECK (condition_out IS NULL OR condition_out IN ('new', 'good', 'fair', 'poor')),

    -- returned_at IS NULL is what makes this row "current". The service
    -- validates the returned_at/returned_by/condition_in triple; a CHECK
    -- pairing them would break DELETE on users (see migration header).
    returned_at     TIMESTAMPTZ,
    returned_by     UUID        REFERENCES users(id) ON DELETE SET NULL,
    condition_in    TEXT        CHECK (condition_in IS NULL OR condition_in IN ('new', 'good', 'fair', 'poor', 'damaged')),
    notes           TEXT,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_asgn_asset    ON hrm_asset_assignments (asset_id, assigned_at DESC);
CREATE INDEX idx_hrm_asgn_employee ON hrm_asset_assignments (employee_id);
CREATE INDEX idx_hrm_asgn_org_id   ON hrm_asset_assignments (org_id);

-- THE constraint that makes "current holder" a well-defined derived query
-- rather than a guess: at most one unreturned assignment per asset.
CREATE UNIQUE INDEX uq_hrm_asgn_active ON hrm_asset_assignments (asset_id)
    WHERE returned_at IS NULL;

COMMENT ON TABLE hrm_asset_assignments IS 'Full assignment history. The row with returned_at IS NULL is the current holder — uq_hrm_asgn_active guarantees there is at most one.';

-- ------------------------------------------------------------
-- hrm_asset_maintenance_logs
-- ------------------------------------------------------------
CREATE TABLE hrm_asset_maintenance_logs (
    id              UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT          NOT NULL UNIQUE
                                      DEFAULT ('amnt_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id          UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    asset_id        UUID          NOT NULL REFERENCES hrm_assets(id) ON DELETE CASCADE,

    maintenance_type TEXT         NOT NULL
                                      CHECK (maintenance_type IN ('repair', 'service', 'upgrade', 'inspection', 'other')),
    description      TEXT,
    cost             NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (cost >= 0),
    currency         CHAR(3)       NOT NULL DEFAULT 'USD',
    performed_at     DATE          NOT NULL,
    vendor           TEXT,

    created_by       UUID          NOT NULL REFERENCES users(id),
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW()
    -- No updated_at — a maintenance record is history, corrected by adding a
    -- new row, the hrm_application_stage_history / hrm_loan_recovery_events shape.
);

CREATE INDEX idx_hrm_amnt_asset  ON hrm_asset_maintenance_logs (asset_id, performed_at DESC);
CREATE INDEX idx_hrm_amnt_org_id ON hrm_asset_maintenance_logs (org_id);

-- ------------------------------------------------------------
-- hrm_asset_requests
-- ------------------------------------------------------------
CREATE TABLE hrm_asset_requests (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id             TEXT        NOT NULL UNIQUE
                                          DEFAULT ('areq_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id           UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,
    category_id           UUID        REFERENCES hrm_asset_categories(id) ON DELETE SET NULL,

    justification         TEXT,
    status                TEXT        NOT NULL DEFAULT 'draft'
                                          CHECK (status IN (
                                              'draft', 'pending_approval', 'approved',
                                              'fulfilled', 'rejected', 'cancelled'
                                          )),
    approval_instance_id  UUID        REFERENCES hrm_approval_instances(id) ON DELETE SET NULL,
    -- Set when an approved request is satisfied by handing over a real asset.
    fulfilled_asset_id    UUID        REFERENCES hrm_assets(id) ON DELETE SET NULL,
    fulfilled_at          TIMESTAMPTZ,

    created_by            UUID        NOT NULL REFERENCES users(id),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_areq_org_id   ON hrm_asset_requests (org_id, status);
CREATE INDEX idx_hrm_areq_employee ON hrm_asset_requests (employee_id);

-- ------------------------------------------------------------
-- hrm_software_licenses
-- ------------------------------------------------------------
CREATE TABLE hrm_software_licenses (
    id            UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id     TEXT          NOT NULL UNIQUE
                                    DEFAULT ('lic_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id        UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name          TEXT          NOT NULL,
    vendor        TEXT,
    seats_total   INTEGER       NOT NULL CHECK (seats_total > 0),
    cost_per_seat NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (cost_per_seat >= 0),
    currency      CHAR(3)       NOT NULL DEFAULT 'USD',
    renewal_date  DATE,
    is_active     BOOLEAN       NOT NULL DEFAULT TRUE,

    created_by    UUID          NOT NULL REFERENCES users(id),
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_lic_org_id ON hrm_software_licenses (org_id, is_active);

COMMENT ON TABLE hrm_software_licenses IS 'A DIFFERENT shape from hrm_assets: N seats, a renewal date, per-seat cost — no single holder, no serial number. There is deliberately NO seats_used column; it is COUNT(*) over unreleased seat assignments.';

-- ------------------------------------------------------------
-- hrm_license_seat_assignments
-- ------------------------------------------------------------
CREATE TABLE hrm_license_seat_assignments (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id    TEXT        NOT NULL UNIQUE
                                 DEFAULT ('lseat_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id       UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    license_id   UUID        NOT NULL REFERENCES hrm_software_licenses(id) ON DELETE CASCADE,
    employee_id  UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    assigned_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_by  UUID        NOT NULL REFERENCES users(id),
    released_at  TIMESTAMPTZ,

    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_lseat_license  ON hrm_license_seat_assignments (license_id);
CREATE INDEX idx_hrm_lseat_employee ON hrm_license_seat_assignments (employee_id);
CREATE INDEX idx_hrm_lseat_org_id   ON hrm_license_seat_assignments (org_id);

-- One live seat per (licence, employee) — re-assigning after release is fine.
CREATE UNIQUE INDEX uq_hrm_lseat_active ON hrm_license_seat_assignments (license_id, employee_id)
    WHERE released_at IS NULL;

-- ------------------------------------------------------------
-- Widen hrm_acknowledgements.acknowledgeable_type for handover sign-off
-- ------------------------------------------------------------
ALTER TABLE hrm_acknowledgements
    DROP CONSTRAINT IF EXISTS hrm_acknowledgements_acknowledgeable_type_check,
    ADD CONSTRAINT hrm_acknowledgements_acknowledgeable_type_check
        CHECK (acknowledgeable_type IN (
            'warning', 'document', 'announcement', 'calendar_event', 'policy',
            'appraisal', 'course_completion', 'asset_handover'
        ));

-- ------------------------------------------------------------
-- Widen BOTH approval CHECKs for asset requests.
--
-- Two constraints, two vocabularies: hrm_approval_TEMPLATES.action_type uses
-- the short form, hrm_approval_INSTANCES.entity_type the long form. Widening
-- only one leaves a template creatable with no instance of it possible (or
-- the reverse). Missed once in 7B, re-hit in 7C — see 00098's header.
-- 'asset_request' happens to read the same in both, which is exactly why it
-- is easy to widen only one and not notice.
-- ------------------------------------------------------------
ALTER TABLE hrm_approval_templates
    DROP CONSTRAINT IF EXISTS hrm_approval_templates_action_type_check,
    ADD CONSTRAINT hrm_approval_templates_action_type_check
        CHECK (action_type IN (
            'leave', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'offer', 'custom',
            'salary_revision', 'bonus', 'loan', 'reimbursement', 'asset_request'
        ));

ALTER TABLE hrm_approval_instances
    DROP CONSTRAINT IF EXISTS hrm_approval_instances_entity_type_check,
    ADD CONSTRAINT hrm_approval_instances_entity_type_check
        CHECK (entity_type IN (
            'leave_request', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'offer', 'custom',
            'salary_revision', 'bonus', 'loan', 'reimbursement', 'asset_request'
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
            'salary_revision', 'bonus', 'loan', 'reimbursement'
        ));

ALTER TABLE hrm_approval_templates
    DROP CONSTRAINT IF EXISTS hrm_approval_templates_action_type_check,
    ADD CONSTRAINT hrm_approval_templates_action_type_check
        CHECK (action_type IN (
            'leave', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'offer', 'custom',
            'salary_revision', 'bonus', 'loan', 'reimbursement'
        ));

ALTER TABLE hrm_acknowledgements
    DROP CONSTRAINT IF EXISTS hrm_acknowledgements_acknowledgeable_type_check,
    ADD CONSTRAINT hrm_acknowledgements_acknowledgeable_type_check
        CHECK (acknowledgeable_type IN (
            'warning', 'document', 'announcement', 'calendar_event', 'policy',
            'appraisal', 'course_completion'
        ));

-- Reverse dependency order: seat assignments before licences, assignments and
-- maintenance and requests before assets, assets before categories. This is
-- NOT the mirror image of the CREATE order above — 6B's 00094 shipped a
-- broken Down block by assuming symmetry, caught only by running it.
DROP TABLE IF EXISTS hrm_license_seat_assignments;
DROP TABLE IF EXISTS hrm_software_licenses;
DROP TABLE IF EXISTS hrm_asset_requests;
DROP TABLE IF EXISTS hrm_asset_maintenance_logs;
DROP TABLE IF EXISTS hrm_asset_assignments;
DROP TABLE IF EXISTS hrm_assets;
DROP TABLE IF EXISTS hrm_asset_categories;

-- +goose StatementEnd
