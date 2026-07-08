-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00029_hrm_promotions
--
-- HRM employee promotion management (Group B1).
--
-- Status machine:
--   draft → pending_approval → approved → applied
--              ↓                  ↓
--           rejected           cancelled
--
-- Design notes:
--   • from_* fields are snapshots taken at record creation — they
--     represent the employee's state BEFORE the promotion.
--   • to_* fields represent what WILL change after apply.
--   • When status = 'applied':
--       employee.position_id   ← to_position_id
--       employee.department_id ← to_department_id (if changing)
--       hrm_employee_salary_records row inserted (if pay changes)
--   • Requires A1 (salary structures), A2 (approval templates),
--     A4 (document templates) to exist first.
-- ============================================================

CREATE TABLE hrm_promotions (
    id                       UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id                TEXT          NOT NULL UNIQUE
                                               DEFAULT ('promo_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                   UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id              UUID          NOT NULL REFERENCES hrm_employees(id) ON DELETE RESTRICT,

    -- Current state snapshot (captured at record creation)
    from_position_id         UUID          REFERENCES hrm_positions(id) ON DELETE SET NULL,
    from_department_id       UUID          REFERENCES hrm_departments(id) ON DELETE SET NULL,
    from_salary_structure_id UUID          REFERENCES hrm_salary_structures(id) ON DELETE SET NULL,
    from_basic_pay           NUMERIC(15,2),

    -- Target state (what changes after apply)
    to_position_id           UUID          NOT NULL REFERENCES hrm_positions(id) ON DELETE RESTRICT,
    to_department_id         UUID          REFERENCES hrm_departments(id) ON DELETE SET NULL,
    to_salary_structure_id   UUID          REFERENCES hrm_salary_structures(id) ON DELETE SET NULL,
    new_basic_pay            NUMERIC(15,2),  -- NULL = no pay change

    effective_date           DATE          NOT NULL,
    reason                   TEXT,
    notes                    TEXT,

    -- A2: approval instance (NULL if no approval required)
    approval_instance_id     UUID          REFERENCES hrm_approval_instances(id) ON DELETE SET NULL,

    -- A4: generated promotion letter
    document_id              UUID          REFERENCES hrm_employee_documents(id) ON DELETE SET NULL,

    status                   TEXT          NOT NULL DEFAULT 'draft'
                                               CHECK (status IN (
                                                   'draft', 'pending_approval', 'approved',
                                                   'rejected', 'cancelled', 'applied'
                                               )),

    applied_at               TIMESTAMPTZ,
    applied_by               UUID          REFERENCES users(id) ON DELETE SET NULL,

    created_by               UUID          NOT NULL REFERENCES users(id),
    created_at               TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_promo_org_id      ON hrm_promotions (org_id);
CREATE INDEX idx_hrm_promo_employee_id ON hrm_promotions (employee_id);
CREATE INDEX idx_hrm_promo_status      ON hrm_promotions (org_id, status);
CREATE INDEX idx_hrm_promo_effective   ON hrm_promotions (effective_date) WHERE status = 'approved';

COMMENT ON TABLE  hrm_promotions IS 'Employee promotion records: position, department, and/or pay change';
COMMENT ON COLUMN hrm_promotions.from_basic_pay IS 'Snapshot of basic pay before promotion; enables audit comparison';
COMMENT ON COLUMN hrm_promotions.new_basic_pay  IS 'NULL = pay is not changing in this promotion';
COMMENT ON COLUMN hrm_promotions.applied_at     IS 'When the employee record was actually updated; NULL until applied';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_promotions;

-- +goose StatementEnd
