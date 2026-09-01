-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00121_hrm_org_chart
--
-- Phase 10A: the org chart. Two tables:
--
--   hrm_reporting_relationships — effective-dated, matrix-capable reporting
--   hrm_position_seats          — a position's seat, occupant optional
--
-- ⚠ hrm_employees.manager_id IS NOT REPLACED, AND KEEPING IT IN SYNC IS THE
-- WHOLE DIFFICULTY OF THIS SLICE.
--
-- That column is the denormalized CURRENT SOLID-LINE pointer, and
-- internal/hrm/scope/predicate.go's view_team tier is a recursive CTE built
-- entirely on it (`he.manager_id = caller.id`, then `JOIN subordinates s ON
-- he.manager_id = s.id`). Every scope-tiered HRM permission in the product —
-- employees, payroll, leave, assets, expenses, exits — resolves through that
-- CTE. Dropping the column, or letting it drift from this table, silently
-- changes who can see whose salary.
--
-- So the direction is fixed: this table is the source of truth, and the
-- service writes manager_id BACK from it whenever the current solid-line
-- relationship changes. A change must never originate on the column.
--
-- Design notes:
--
--   • EFFECTIVE-DATED, so "who did this person report to in March" is
--     answerable — which is what makes an org chart useful for an appraisal
--     cycle or an audit rather than just a current-state picture. A
--     relationship is ended by stamping effective_to, never by deleting the
--     row: deleting one destroys the history the table exists to hold.
--
--   • relationship_type SEPARATES AUTHORIZATION FROM MATRIX REPORTING. Only
--     'solid' feeds manager_id and therefore view_team. 'dotted',
--     'functional' and 'project' are real reporting lines an org chart must
--     draw, but they must NOT widen anybody's data access — a project lead
--     seeing their contributors' pay because of a dotted line would be a
--     quiet privilege escalation.
--
--   • CYCLE DETECTION lives in the service, not here. A CHECK can stop the
--     degenerate self-reference (below) but cannot see A->B->C->A; that needs
--     a recursive walk, and the service refuses before inserting. The DB
--     guard is the floor, not the mechanism.
--
--   • hrm_position_seats.employee_id IS NULLABLE ON PURPOSE. A vacant seat is
--     the thing a requisition is raised against, so the retrofit that turns
--     "seat 3 of Senior Engineer is empty" into a Phase 4 job requisition
--     needs no schema change.
--
-- What must NEVER be added (the 00076 CHECK x ON DELETE SET NULL trap):
-- Postgres re-evaluates CHECKs on UPDATE and ON DELETE SET NULL *is* an
-- UPDATE. hrm_position_seats.employee_id is ON DELETE SET NULL, so
--   CHECK (is_occupied = FALSE OR employee_id IS NOT NULL)
-- would make DELETE FROM hrm_employees fail 23514 for any occupied seat —
-- which is also why there is no is_occupied column: occupancy is
-- employee_id IS NOT NULL, derived, per the 00076 rule.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_reporting_relationships
-- ------------------------------------------------------------
CREATE TABLE hrm_reporting_relationships (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id         TEXT        NOT NULL UNIQUE
                                      DEFAULT ('rrel_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id            UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Both ends CASCADE: a relationship has no meaning once either party is
    -- deleted, and CASCADE (rather than SET NULL) is what keeps the
    -- self-reference CHECK below safe from the 00076 trap.
    employee_id       UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,
    manager_id        UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    relationship_type TEXT        NOT NULL DEFAULT 'solid'
                                      CHECK (relationship_type IN (
                                          'solid', 'dotted', 'functional', 'project'
                                      )),

    effective_from    DATE        NOT NULL DEFAULT CURRENT_DATE,
    -- NULL = still in force. Ending a relationship stamps this; it never
    -- deletes the row, because the history is the point of the table.
    effective_to      DATE,

    note              TEXT,
    created_by        UUID        NOT NULL REFERENCES users(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- The degenerate cycle, refused at the floor. Everything longer than one
    -- hop is the service's job.
    CONSTRAINT chk_hrm_rrel_not_self CHECK (employee_id <> manager_id),
    CONSTRAINT chk_hrm_rrel_dates    CHECK (effective_to IS NULL OR effective_to >= effective_from)
);

-- ONE ACTIVE SOLID MANAGER PER EMPLOYEE. Two would make manager_id ambiguous
-- and the view_team CTE non-deterministic. Matrix lines are unconstrained —
-- an employee may hold several dotted or project relationships at once, which
-- is the entire reason those types exist.
CREATE UNIQUE INDEX uq_hrm_rrel_active_solid
    ON hrm_reporting_relationships (employee_id)
    WHERE effective_to IS NULL AND relationship_type = 'solid';

CREATE INDEX idx_hrm_rrel_org       ON hrm_reporting_relationships (org_id, relationship_type);
CREATE INDEX idx_hrm_rrel_employee  ON hrm_reporting_relationships (employee_id, effective_from DESC);
CREATE INDEX idx_hrm_rrel_manager   ON hrm_reporting_relationships (manager_id)
    WHERE effective_to IS NULL;

COMMENT ON TABLE hrm_reporting_relationships IS 'Effective-dated, matrix-capable reporting. The SOURCE OF TRUTH for reporting lines; hrm_employees.manager_id is a denormalized copy of the current solid line, written back by the service because scope.Predicate''s view_team CTE depends on it.';
COMMENT ON COLUMN hrm_reporting_relationships.relationship_type IS 'Only ''solid'' feeds hrm_employees.manager_id and therefore view_team. Dotted/functional/project are drawn on the chart but must never widen data access.';

-- ------------------------------------------------------------
-- hrm_position_seats
-- ------------------------------------------------------------
CREATE TABLE hrm_position_seats (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id    TEXT        NOT NULL UNIQUE
                                 DEFAULT ('seat_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id       UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    position_id  UUID        NOT NULL REFERENCES hrm_positions(id) ON DELETE CASCADE,

    -- NULLABLE, deliberately: a vacant seat is what a requisition is raised
    -- against. There is no is_occupied column — occupancy is
    -- employee_id IS NOT NULL, derived on read (the 00076 rule).
    employee_id  UUID        REFERENCES hrm_employees(id) ON DELETE SET NULL,

    seat_label   TEXT,
    is_active    BOOLEAN     NOT NULL DEFAULT TRUE,

    created_by   UUID        NOT NULL REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- An employee occupies at most one active seat. Two would make headcount
-- double-count them in Phase 10C's snapshots.
CREATE UNIQUE INDEX uq_hrm_seat_employee
    ON hrm_position_seats (employee_id)
    WHERE employee_id IS NOT NULL AND is_active = TRUE;

CREATE INDEX idx_hrm_seat_position ON hrm_position_seats (position_id, is_active);
CREATE INDEX idx_hrm_seat_vacant   ON hrm_position_seats (org_id)
    WHERE employee_id IS NULL AND is_active = TRUE;

COMMENT ON TABLE hrm_position_seats IS 'A seat on a position. employee_id is nullable so a VACANT seat is representable — that is what a future requisition is raised against, with no schema change needed.';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Independent of each other; neither is referenced by anything else.
DROP TABLE IF EXISTS hrm_position_seats;
DROP TABLE IF EXISTS hrm_reporting_relationships;

-- hrm_employees.manager_id is untouched by this migration in either
-- direction — it existed long before and every scope tier depends on it.

-- +goose StatementEnd
