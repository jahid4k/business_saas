-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00021_create_hrm_tables
--
-- Human Resource Management tables:
--   hrm_departments     — department hierarchy
--   hrm_positions       — job title catalog
--   hrm_employees       — core employee records
--   hrm_leave_types     — configurable leave type catalog
--   hrm_leave_requests  — employee leave requests with approval flow
--
-- Design notes:
--   • All HRM tables are org-scoped (org_id = tenant isolation).
--   • hrm_departments.parent_department_id is nullable for top-level depts.
--   • hrm_departments.head_employee_id FK is deferred until after
--     hrm_employees exists (chicken-and-egg pattern, same as crm_leads → crm_deals).
--   • hrm_employees.manager_id is a self-FK added after table creation.
--   • hrm_employees.user_id is nullable — employees do not have to be
--     platform users; useful for contractors or imported headcount.
--   • leave_requests.total_days uses NUMERIC(5,1) for half-day support.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_departments
-- ------------------------------------------------------------
CREATE TABLE hrm_departments (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id            TEXT        NOT NULL UNIQUE
                                         DEFAULT ('dept_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id               UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name                 TEXT        NOT NULL,
    description          TEXT,
    parent_department_id UUID        REFERENCES hrm_departments(id) ON DELETE SET NULL,
    head_employee_id     UUID,       -- FK added below after hrm_employees exists

    is_active            BOOLEAN     NOT NULL DEFAULT TRUE,

    created_by           UUID        NOT NULL REFERENCES users(id),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_dept_org_id     ON hrm_departments (org_id);
CREATE INDEX idx_hrm_dept_parent_id  ON hrm_departments (parent_department_id);
CREATE UNIQUE INDEX idx_hrm_dept_org_name ON hrm_departments (org_id, LOWER(name)) WHERE is_active = TRUE;

COMMENT ON TABLE hrm_departments IS 'HRM department hierarchy, scoped to an organization';
COMMENT ON COLUMN hrm_departments.parent_department_id IS 'NULL for top-level (root) departments';
COMMENT ON COLUMN hrm_departments.head_employee_id     IS 'Department head — FK to hrm_employees added after that table exists';

-- ------------------------------------------------------------
-- hrm_positions
-- ------------------------------------------------------------
CREATE TABLE hrm_positions (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id     TEXT        NOT NULL UNIQUE
                                  DEFAULT ('pos_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id        UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID        REFERENCES hrm_departments(id) ON DELETE SET NULL,

    title         TEXT        NOT NULL,
    description   TEXT,
    is_active     BOOLEAN     NOT NULL DEFAULT TRUE,

    created_by    UUID        NOT NULL REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_pos_org_id        ON hrm_positions (org_id);
CREATE INDEX idx_hrm_pos_department_id ON hrm_positions (department_id);
CREATE UNIQUE INDEX idx_hrm_pos_org_title ON hrm_positions (org_id, LOWER(title)) WHERE is_active = TRUE;

COMMENT ON TABLE hrm_positions IS 'Job title catalog, optionally linked to a department';

-- ------------------------------------------------------------
-- hrm_employees
-- ------------------------------------------------------------
CREATE TABLE hrm_employees (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id        TEXT        NOT NULL UNIQUE
                                     DEFAULT ('emp_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id           UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Optional link to a platform user account
    user_id          UUID        REFERENCES users(id) ON DELETE SET NULL,

    -- Identity
    employee_number  TEXT,
    first_name       TEXT        NOT NULL,
    last_name        TEXT,
    email            TEXT,
    work_email       TEXT,
    phone            TEXT,
    work_phone       TEXT,
    date_of_birth    DATE,
    gender           TEXT        CHECK (gender IN ('male', 'female', 'other', 'prefer_not_to_say')),
    avatar_url       TEXT,

    -- Employment
    hire_date        DATE        NOT NULL,
    termination_date DATE,
    employment_type  TEXT        NOT NULL DEFAULT 'full_time'
                                     CHECK (employment_type IN ('full_time', 'part_time', 'contractor', 'intern')),
    status           TEXT        NOT NULL DEFAULT 'active'
                                     CHECK (status IN ('active', 'inactive', 'on_leave', 'terminated')),

    -- Org chart
    department_id    UUID        REFERENCES hrm_departments(id) ON DELETE SET NULL,
    position_id      UUID        REFERENCES hrm_positions(id) ON DELETE SET NULL,
    manager_id       UUID,       -- self-FK added after table creation

    -- Address
    address          TEXT,
    city             TEXT,
    country          TEXT,

    -- Misc
    notes            TEXT,

    created_by       UUID        NOT NULL REFERENCES users(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_emp_org_id        ON hrm_employees (org_id);
CREATE INDEX idx_hrm_emp_user_id       ON hrm_employees (user_id)       WHERE user_id IS NOT NULL;
CREATE INDEX idx_hrm_emp_department_id ON hrm_employees (department_id) WHERE department_id IS NOT NULL;
CREATE INDEX idx_hrm_emp_position_id   ON hrm_employees (position_id)   WHERE position_id IS NOT NULL;
CREATE INDEX idx_hrm_emp_manager_id    ON hrm_employees (manager_id)    WHERE manager_id IS NOT NULL;
CREATE INDEX idx_hrm_emp_status        ON hrm_employees (status);
CREATE UNIQUE INDEX idx_hrm_emp_org_emp_number ON hrm_employees (org_id, employee_number)
    WHERE employee_number IS NOT NULL;

COMMENT ON TABLE hrm_employees IS 'Core employee records; user_id is nullable for non-platform users';
COMMENT ON COLUMN hrm_employees.user_id         IS 'Optional link to platform user; contractors may not have one';
COMMENT ON COLUMN hrm_employees.employee_number IS 'Org-scoped HR employee identifier (e.g. EMP-001)';

-- Self-FK for reporting hierarchy
ALTER TABLE hrm_employees
    ADD CONSTRAINT fk_hrm_emp_manager
    FOREIGN KEY (manager_id) REFERENCES hrm_employees(id) ON DELETE SET NULL;

-- Now add the deferred FK for department head
ALTER TABLE hrm_departments
    ADD CONSTRAINT fk_hrm_dept_head
    FOREIGN KEY (head_employee_id) REFERENCES hrm_employees(id) ON DELETE SET NULL;

-- ------------------------------------------------------------
-- hrm_leave_types
-- ------------------------------------------------------------
CREATE TABLE hrm_leave_types (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id         TEXT        NOT NULL UNIQUE
                                      DEFAULT ('lvt_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id            UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name              TEXT        NOT NULL,
    description       TEXT,
    max_days_per_year INTEGER     NOT NULL DEFAULT 0  -- 0 = unlimited
                                      CHECK (max_days_per_year >= 0),
    is_paid           BOOLEAN     NOT NULL DEFAULT TRUE,
    requires_approval BOOLEAN     NOT NULL DEFAULT TRUE,
    is_active         BOOLEAN     NOT NULL DEFAULT TRUE,

    created_by        UUID        NOT NULL REFERENCES users(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_lvt_org_id ON hrm_leave_types (org_id);
CREATE UNIQUE INDEX idx_hrm_lvt_org_name ON hrm_leave_types (org_id, LOWER(name)) WHERE is_active = TRUE;

COMMENT ON TABLE hrm_leave_types IS 'Configurable leave type catalog per organization';
COMMENT ON COLUMN hrm_leave_types.max_days_per_year IS '0 means unlimited days per year';

-- ------------------------------------------------------------
-- hrm_leave_requests
-- ------------------------------------------------------------
CREATE TABLE hrm_leave_requests (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT         NOT NULL UNIQUE
                                     DEFAULT ('lvr_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id          UUID         NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    employee_id     UUID         NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,
    leave_type_id   UUID         NOT NULL REFERENCES hrm_leave_types(id) ON DELETE RESTRICT,

    start_date      DATE         NOT NULL,
    end_date        DATE         NOT NULL,
    total_days      NUMERIC(5,1) NOT NULL CHECK (total_days > 0),

    reason          TEXT,
    status          TEXT         NOT NULL DEFAULT 'pending'
                                     CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled')),

    reviewed_by     UUID         REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at     TIMESTAMPTZ,
    review_note     TEXT,

    created_by      UUID         NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_lvr_dates CHECK (end_date >= start_date)
);

CREATE INDEX idx_hrm_lvr_org_id       ON hrm_leave_requests (org_id);
CREATE INDEX idx_hrm_lvr_employee_id  ON hrm_leave_requests (employee_id);
CREATE INDEX idx_hrm_lvr_leave_type   ON hrm_leave_requests (leave_type_id);
CREATE INDEX idx_hrm_lvr_status       ON hrm_leave_requests (status);
CREATE INDEX idx_hrm_lvr_start_date   ON hrm_leave_requests (start_date);

COMMENT ON TABLE hrm_leave_requests IS 'Employee leave requests with approval workflow';
COMMENT ON COLUMN hrm_leave_requests.total_days IS 'Supports half-days via NUMERIC(5,1), e.g. 0.5';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE hrm_departments  DROP CONSTRAINT IF EXISTS fk_hrm_dept_head;
ALTER TABLE hrm_employees    DROP CONSTRAINT IF EXISTS fk_hrm_emp_manager;

DROP TABLE IF EXISTS hrm_leave_requests;
DROP TABLE IF EXISTS hrm_leave_types;
DROP TABLE IF EXISTS hrm_employees;
DROP TABLE IF EXISTS hrm_positions;
DROP TABLE IF EXISTS hrm_departments;

-- +goose StatementEnd
