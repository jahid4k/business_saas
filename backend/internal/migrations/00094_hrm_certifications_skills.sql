-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00094_hrm_certifications_skills
--
-- Phase 6B: certifications, the shared skills taxonomy, and course
-- auto-assignment. Five tables:
--   hrm_certifications          — the catalogue
--   hrm_employee_certifications — one person's credential, WITH expiry
--   hrm_skills                  — the ORG-LEVEL shared taxonomy
--   hrm_employee_skills         — who has which skill, and where it came from
--   hrm_enrollment_rules        — simple auto-assignment rows
--
-- Plus one CHECK widening: hrm_acknowledgements.acknowledgeable_type gains
-- 'course_completion', so compliance evidence reuses the existing
-- acknowledgement machinery rather than growing a parallel one. Exactly the
-- move migration 00086 made for 'appraisal', and the build plan schedules
-- this one by name.
--
-- ⚠ TABLE ORDER MATTERS HERE: hrm_skills is created BEFORE hrm_certifications,
-- because hrm_certifications.skill_id references it. Reordering these blocks
-- into "logical" order breaks the migration with an undefined-table error.
--
-- Design notes:
--
--   • hrm_position_skills IS DELIBERATELY NOT BUILT. The build plan lists it
--     alongside the other two, but skills a POSITION requires has no reader
--     until Phase 10's succession and gap analysis. Recruitment and
--     performance were both grepped at design time and contain zero skills
--     fields, so there is nothing to retrofit into either. Building it now
--     would be precisely the speculative primitive this plan's rule 1 exists
--     to prevent. hrm_skills and hrm_employee_skills DO earn their place,
--     because course and certification completion granting a skill is a real
--     in-phase consumer.
--
--     What the build plan's "shared taxonomy, not an LMS-internal table" note
--     is really about is PLACEMENT, and that is honoured: hrm_skills is
--     org-scoped and names nothing about courses, so Phase 10 can adopt it
--     without a migration.
--
--   • hrm_employee_certifications.expires_at is the point of the whole table.
--     The build plan calls the expiry sweep "the highest-value feature here".
--     It is a DATE, indexed partially on the open statuses, because the sweep
--     queries exactly that slice nightly.
--
--   • enrollment_id is NULLABLE, and that is the common case rather than an
--     edge case: an externally-obtained professional licence is a
--     certification with no course behind it. ON DELETE SET NULL so purging
--     an old enrollment does not erase the credential it produced.
--
--   • hrm_employee_skills.source records HOW a skill was acquired, with
--     nullable source ids for each origin. A skill granted by a course keeps
--     a pointer to the enrollment; one entered by hand keeps none.
--
--   • hrm_enrollment_rules is SIMPLE RULE ROWS, not a rule engine — the build
--     plan says so explicitly, and it is the easiest thing here to over-build.
--     One row = one (course, trigger) pair with an optional department or
--     position filter. No boolean composition, no priorities, no scripting.
--
-- What must NEVER be added here (the 00076 CHECK × ON DELETE SET NULL trap):
-- Postgres re-evaluates CHECKs on UPDATE and ON DELETE SET NULL *is* an
-- UPDATE, so a CHECK pairing two columns breaks DELETE on the referenced
-- table. Specifically:
--   • CHECK (source <> 'course' OR source_enrollment_id IS NOT NULL) on
--     hrm_employee_skills would make DELETE FROM hrm_enrollments fail 23514.
--   • CHECK (issued_via <> 'course' OR enrollment_id IS NOT NULL) on
--     hrm_employee_certifications would do the same.
-- Both facts are carried by nullable columns and validated in the service.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_skills — the ORG-LEVEL shared taxonomy.
--
-- Names nothing about courses on purpose: Phase 10's succession and gap
-- analysis adopt this table without a migration. That placement, not the
-- table count, is what the build plan's "not an LMS-internal table" means.
-- ------------------------------------------------------------
CREATE TABLE hrm_skills (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE
                                DEFAULT ('skl_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name        TEXT        NOT NULL,
    description TEXT,
    category    TEXT,

    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    created_by  UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_hrm_skill_org_name ON hrm_skills (org_id, LOWER(name));
CREATE INDEX idx_hrm_skill_org_active ON hrm_skills (org_id, is_active);

COMMENT ON TABLE hrm_skills IS 'Org-level shared taxonomy. Phase 10 succession reads this directly; hrm_position_skills is deliberately deferred until it has a reader';

-- ------------------------------------------------------------
-- hrm_certifications — the catalogue.
-- ------------------------------------------------------------
CREATE TABLE hrm_certifications (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id    TEXT        NOT NULL UNIQUE
                                 DEFAULT ('cert_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id       UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name         TEXT        NOT NULL,
    description  TEXT,
    issuing_body TEXT,

    -- Default validity. NULL means the credential does not expire, which is
    -- different from expiring today and must stay distinguishable.
    validity_months INTEGER  CHECK (validity_months IS NULL OR validity_months > 0),

    -- Optional link to the course that grants it, so completing the course
    -- can issue the credential. SET NULL: deleting a course must not destroy
    -- the catalogue entry, which may also be earned externally.
    course_id    UUID        REFERENCES hrm_courses(id) ON DELETE SET NULL,

    -- Optional skill this credential demonstrates. Issuing the credential
    -- records that skill against the employee, which is the in-phase consumer
    -- that justifies building the taxonomy now rather than in Phase 10.
    -- SET NULL: retiring a skill must not invalidate the certification.
    skill_id     UUID        REFERENCES hrm_skills(id) ON DELETE SET NULL,

    is_active    BOOLEAN     NOT NULL DEFAULT TRUE,
    created_by   UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_hrm_cert_org_name ON hrm_certifications (org_id, LOWER(name));
CREATE INDEX idx_hrm_cert_org_active ON hrm_certifications (org_id, is_active);
CREATE INDEX idx_hrm_cert_course ON hrm_certifications (course_id) WHERE course_id IS NOT NULL;
CREATE INDEX idx_hrm_cert_skill ON hrm_certifications (skill_id) WHERE skill_id IS NOT NULL;

COMMENT ON COLUMN hrm_certifications.validity_months IS 'NULL means never expires — distinct from expiring today, and the sweep must keep them distinguishable';

-- ------------------------------------------------------------
-- hrm_employee_certifications — one person's credential.
-- ------------------------------------------------------------
CREATE TABLE hrm_employee_certifications (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id        TEXT        NOT NULL UNIQUE
                                     DEFAULT ('ecrt_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id           UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- NOT NULL is load-bearing: scope.Predicate emits
    -- employee_id = (SELECT ...), and NULL makes that expression NULL rather
    -- than FALSE, so the row would vanish from every non-ScopeAll list
    -- instead of being denied. The hrm_goals.employee_id precedent.
    employee_id      UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,
    certification_id UUID        NOT NULL REFERENCES hrm_certifications(id) ON DELETE RESTRICT,

    -- Nullable, and this is the COMMON case: an externally-obtained
    -- professional licence has no course behind it.
    enrollment_id    UUID        REFERENCES hrm_enrollments(id) ON DELETE SET NULL,

    credential_id    TEXT,
    issued_on        DATE        NOT NULL,
    -- The point of this table. NULL means it does not expire.
    expires_at       DATE,

    status           TEXT        NOT NULL DEFAULT 'active'
                     CHECK (status IN ('active', 'expiring', 'expired', 'revoked')),

    -- Stamped by the sweep so a reminder is not sent every night.
    expiry_notified_at TIMESTAMPTZ,

    notes            TEXT,
    issued_by        UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_ecrt_expiry CHECK (expires_at IS NULL OR expires_at >= issued_on)
);

-- One live credential per employee per certification. Partial, so a revoked
-- or expired one frees them to re-certify — which is the whole renewal path.
CREATE UNIQUE INDEX uq_hrm_ecrt_employee_cert_live
    ON hrm_employee_certifications (employee_id, certification_id)
    WHERE status IN ('active', 'expiring');

CREATE INDEX idx_hrm_ecrt_employee ON hrm_employee_certifications (employee_id, status);
CREATE INDEX idx_hrm_ecrt_org_status ON hrm_employee_certifications (org_id, status);
-- The sweep's access path: exactly the slice it scans nightly.
CREATE INDEX idx_hrm_ecrt_expiry ON hrm_employee_certifications (expires_at)
    WHERE expires_at IS NOT NULL AND status IN ('active', 'expiring');

COMMENT ON TABLE hrm_employee_certifications IS 'One credential; expires_at drives the nightly expiry sweep, the highest-value feature in Phase 6';
COMMENT ON COLUMN hrm_employee_certifications.enrollment_id IS 'Nullable BY DESIGN — an externally-obtained licence has no course; SET NULL so purging an enrollment does not erase the credential';

-- ------------------------------------------------------------
-- hrm_employee_skills — who has which skill, and where it came from.
-- ------------------------------------------------------------
CREATE TABLE hrm_employee_skills (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE
                                DEFAULT ('eskl_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,
    skill_id    UUID        NOT NULL REFERENCES hrm_skills(id) ON DELETE CASCADE,

    proficiency TEXT        CHECK (proficiency IN ('beginner', 'intermediate', 'advanced', 'expert')),

    -- How it was acquired. The source ids are nullable and independent: a
    -- manually recorded skill has neither.
    source                   TEXT NOT NULL DEFAULT 'manual'
                             CHECK (source IN ('manual', 'course', 'certification')),
    source_enrollment_id     UUID REFERENCES hrm_enrollments(id) ON DELETE SET NULL,
    source_certification_id  UUID REFERENCES hrm_employee_certifications(id) ON DELETE SET NULL,

    acquired_on DATE,
    notes       TEXT,
    created_by  UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_hrm_eskl_employee_skill ON hrm_employee_skills (employee_id, skill_id);
CREATE INDEX idx_hrm_eskl_employee ON hrm_employee_skills (employee_id);
CREATE INDEX idx_hrm_eskl_skill ON hrm_employee_skills (skill_id);

-- ------------------------------------------------------------
-- hrm_enrollment_rules — simple auto-assignment rows.
--
-- NOT a rule engine. One row = one (course, trigger) pair with at most one
-- department or position filter. No boolean composition, no priorities, no
-- scripting — the build plan is explicit, and this is the easiest thing in
-- Phase 6 to over-build.
-- ------------------------------------------------------------
CREATE TABLE hrm_enrollment_rules (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE
                                DEFAULT ('erul_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    course_id   UUID        NOT NULL REFERENCES hrm_courses(id) ON DELETE CASCADE,

    -- 'on_hire' fires for a new employee; the other two are swept over
    -- existing staff when the rule is applied.
    trigger     TEXT        NOT NULL
                CHECK (trigger IN ('on_hire', 'department', 'position')),

    department_id UUID      REFERENCES hrm_departments(id) ON DELETE CASCADE,
    position_id   UUID      REFERENCES hrm_positions(id) ON DELETE CASCADE,

    -- Days from assignment to the due date. NULL means no due date.
    due_in_days INTEGER     CHECK (due_in_days IS NULL OR due_in_days >= 0),

    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    created_by  UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_erul_org_active ON hrm_enrollment_rules (org_id, is_active);
CREATE INDEX idx_hrm_erul_course ON hrm_enrollment_rules (course_id);

COMMENT ON TABLE hrm_enrollment_rules IS 'Simple rule rows, NOT a rule engine — one course, one trigger, at most one filter. Per the build plan';

-- ------------------------------------------------------------
-- hrm_acknowledgements: register 'course_completion'
--
-- Compliance evidence reuses the existing acknowledgement machinery. Migration
-- 00086 made the identical widening for 'appraisal'; the build plan schedules
-- this one by name.
-- ------------------------------------------------------------
ALTER TABLE hrm_acknowledgements
    DROP CONSTRAINT IF EXISTS hrm_acknowledgements_acknowledgeable_type_check,
    ADD CONSTRAINT hrm_acknowledgements_acknowledgeable_type_check
        CHECK (acknowledgeable_type IN (
            'warning', 'document', 'announcement', 'calendar_event', 'policy',
            'appraisal', 'course_completion'
        ));

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE hrm_acknowledgements
    DROP CONSTRAINT IF EXISTS hrm_acknowledgements_acknowledgeable_type_check,
    ADD CONSTRAINT hrm_acknowledgements_acknowledgeable_type_check
        CHECK (acknowledgeable_type IN (
            'warning', 'document', 'announcement', 'calendar_event', 'policy', 'appraisal'
        ));

-- Reverse dependency order, and it is not the mirror image of the CREATE
-- order: hrm_certifications.skill_id references hrm_skills, so certifications
-- must go FIRST and skills LAST. Dropping skills earlier fails 2BP01.
DROP TABLE IF EXISTS hrm_enrollment_rules;
DROP TABLE IF EXISTS hrm_employee_skills;
DROP TABLE IF EXISTS hrm_employee_certifications;
DROP TABLE IF EXISTS hrm_certifications;
DROP TABLE IF EXISTS hrm_skills;

-- +goose StatementEnd
