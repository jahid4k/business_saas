-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00092_hrm_learning
--
-- Phase 6A: the LMS core. Eight tables:
--   hrm_courses            — the course shell; owns no content
--   hrm_course_versions    — versioned content root
--   hrm_course_modules     — belong to a VERSION, not a course
--   hrm_course_lessons     — belong to a module
--   hrm_enrollments        — one learner on one PINNED version
--   hrm_lesson_progress    — per-lesson state within an enrollment
--   hrm_quiz_attempts      — a graded run at a quiz lesson
--   hrm_quiz_answer_keys   — the correct answers, owned HERE, never by the
--                            platform form engine
--
-- Design notes:
--
--   • CONTENT HANGS OFF THE VERSION, NOT THE COURSE. hrm_course_modules
--     references hrm_course_versions, and hrm_enrollments pins version_id
--     alongside course_id. The build plan requires the pin, and the reason is
--     migration 00086's: editing a course must not retroactively change what
--     somebody was already assessed on. A course whose modules were mutable
--     in place would rewrite the history of every completed enrollment.
--
--     The form engine already snapshots question definitions onto each
--     instance (00084), so quiz CONTENT is covered by that. Version pinning
--     is what covers course STRUCTURE — which modules and lessons existed,
--     and in what order.
--
--   • THE ANSWER KEY LIVES HERE, NOT IN platform_form_questions.
--     The build plan says "assessments reuse Phase 5's form engine" with
--     "separate DTOs: QuestionForAttempt never carries the correct answer".
--     But the engine has no correct-answer column and no pass mark — its
--     computeScore normalises each answer 0-1 against its own scale and
--     weights it, which is a RATING score, not an ASSESSMENT score.
--
--     Putting the key in a Phase 6 table keeps assessment semantics out of a
--     platform primitive that two non-assessment consumers (appraisals, 360
--     feedback) already depend on — neither should carry a correct_answer
--     column it never reads. It also makes the "never leak the correct
--     answer" rule STRUCTURAL rather than disciplinary: the key is in a table
--     the attempt query does not join, the same shape 5C used to make 360
--     anonymity enforceable rather than merely intended.
--
--   • GRADE ONCE, STORE THE RESULT — never re-derive.
--     platform_form_responses.question_id is ON DELETE SET NULL and is
--     documented there as "provenance only, never joined for display"; the
--     question snapshot lives on the response row itself. So an answer key
--     keyed on question_id becomes unreachable the moment a question is
--     deleted, and a re-grade would silently score zero. hrm_quiz_attempts
--     therefore stores score/passed at submit time, while the question still
--     exists. Same publish-immutability reasoning as hrm_appraisals
--     snapshotting its scores at publish.
--
--   • Lessons carry content_url / content_text DIRECTLY. There is no
--     org-level document table to reference — hrm_employee_documents is
--     employee-scoped, which is the wrong owner for course material. No SCORM
--     player and no video hosting, per the build plan.
--
--   • Completion percentage is COMPUTED from hrm_lesson_progress, never
--     stored. Migration 00076's standing rule, and the platform_checklists
--     precedent: no denormalized counter to drift.
--
-- What must NEVER be added here (the 00076 CHECK × ON DELETE SET NULL trap):
-- Postgres re-evaluates CHECKs on UPDATE, and ON DELETE SET NULL *is* an
-- UPDATE, so a CHECK pairing two columns breaks DELETE on the referenced
-- table. Specifically:
--   • CHECK (lesson_type <> 'quiz' OR form_template_id IS NOT NULL) on
--     hrm_course_lessons would make DELETE FROM platform_form_templates fail
--     23514 for any org with a quiz lesson.
--   • CHECK (passed IS NULL OR form_instance_id IS NOT NULL) on
--     hrm_quiz_attempts would do the same to platform_form_instances.
-- Both facts are carried by nullable columns and enforced in the service.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_courses — the shell. Title and catalogue metadata only.
-- ------------------------------------------------------------
CREATE TABLE hrm_courses (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE
                                DEFAULT ('crs_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    title       TEXT        NOT NULL,
    description TEXT,
    -- Free text for now. The shared hrm_skills taxonomy arrives in 6B; a
    -- courses-local category table would be the speculative primitive the
    -- build plan's rule 1 exists to prevent.
    category    TEXT,

    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,

    created_by  UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_hrm_course_org_title ON hrm_courses (org_id, LOWER(title));
CREATE INDEX idx_hrm_course_org_active ON hrm_courses (org_id, is_active);

COMMENT ON TABLE hrm_courses IS 'Course shell; all content hangs off hrm_course_versions so an edit cannot rewrite a completed enrollment';

-- ------------------------------------------------------------
-- hrm_course_versions — the content root.
--
-- draft → published → archived. Only a published version can be enrolled
-- against; only a draft version can be edited. That pair is what makes the
-- pin on hrm_enrollments meaningful.
-- ------------------------------------------------------------
CREATE TABLE hrm_course_versions (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id      TEXT        NOT NULL UNIQUE
                                   DEFAULT ('crsv_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id         UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    course_id      UUID        NOT NULL REFERENCES hrm_courses(id) ON DELETE CASCADE,

    version_number INTEGER     NOT NULL CHECK (version_number >= 1),
    -- Snapshotted from the course at publish so an archived version still
    -- renders under the title it was published as.
    title_snapshot TEXT        NOT NULL,
    change_note    TEXT,

    status         TEXT        NOT NULL DEFAULT 'draft'
                   CHECK (status IN ('draft', 'published', 'archived')),

    -- Fraction of lessons required to complete the course, per version so
    -- tightening it is a data change on a NEW version rather than a silent
    -- reinterpretation of finished enrollments.
    pass_threshold NUMERIC(5,2) NOT NULL DEFAULT 100
                   CHECK (pass_threshold > 0 AND pass_threshold <= 100),

    published_at   TIMESTAMPTZ,
    published_by   UUID        REFERENCES users(id) ON DELETE SET NULL,
    archived_at    TIMESTAMPTZ,
    created_by     UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_hrm_crsv_course_number ON hrm_course_versions (course_id, version_number);
-- At most one draft per course: two concurrent drafts make "the next version"
-- ambiguous and let two authors silently overwrite each other.
CREATE UNIQUE INDEX uq_hrm_crsv_one_draft ON hrm_course_versions (course_id)
    WHERE status = 'draft';
CREATE INDEX idx_hrm_crsv_course_status ON hrm_course_versions (course_id, status);

COMMENT ON COLUMN hrm_course_versions.pass_threshold IS 'Percent of lessons required to complete; per-version so tightening it cannot reinterpret finished enrollments';

-- ------------------------------------------------------------
-- hrm_course_modules — belong to a VERSION.
-- ------------------------------------------------------------
CREATE TABLE hrm_course_modules (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id     TEXT        NOT NULL UNIQUE
                                  DEFAULT ('crsm_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    version_id    UUID        NOT NULL REFERENCES hrm_course_versions(id) ON DELETE CASCADE,

    title         TEXT        NOT NULL,
    description   TEXT,
    display_order INTEGER     NOT NULL DEFAULT 0,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_crsm_version ON hrm_course_modules (version_id, display_order);

-- ------------------------------------------------------------
-- hrm_course_lessons
--
-- No org_id: reached via module → version, the hrm_goal_checkins /
-- platform_form_responses precedent.
-- ------------------------------------------------------------
CREATE TABLE hrm_course_lessons (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id     TEXT        NOT NULL UNIQUE
                                  DEFAULT ('crsl_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    module_id     UUID        NOT NULL REFERENCES hrm_course_modules(id) ON DELETE CASCADE,

    title         TEXT        NOT NULL,
    lesson_type   TEXT        NOT NULL
                  CHECK (lesson_type IN ('link', 'pdf', 'text', 'quiz')),

    -- Carried directly rather than through a document FK: there is no
    -- org-level document table, and hrm_employee_documents is employee-scoped
    -- — the wrong owner for course material.
    content_url   TEXT,
    content_text  TEXT,

    -- Quiz lessons only. SET NULL rather than RESTRICT so deleting a template
    -- is not blocked; the service refuses to serve a quiz whose template
    -- vanished, which is a better failure than an undeletable template.
    form_template_id UUID     REFERENCES platform_form_templates(id) ON DELETE SET NULL,
    -- Percent required to pass this quiz.
    pass_mark     NUMERIC(5,2) CHECK (pass_mark IS NULL OR (pass_mark >= 0 AND pass_mark <= 100)),
    -- NULL means unlimited retries.
    max_attempts  INTEGER     CHECK (max_attempts IS NULL OR max_attempts >= 1),

    is_required   BOOLEAN     NOT NULL DEFAULT TRUE,
    display_order INTEGER     NOT NULL DEFAULT 0,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_crsl_module ON hrm_course_lessons (module_id, display_order);
CREATE INDEX idx_hrm_crsl_template ON hrm_course_lessons (form_template_id)
    WHERE form_template_id IS NOT NULL;

COMMENT ON COLUMN hrm_course_lessons.form_template_id IS 'Quiz lessons only; SET NULL so a template delete is not blocked, and the service refuses to serve a quiz whose template vanished';

-- ------------------------------------------------------------
-- hrm_enrollments — one learner, one PINNED version.
-- ------------------------------------------------------------
CREATE TABLE hrm_enrollments (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE
                                DEFAULT ('enr_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- NOT NULL is load-bearing: scope.Predicate emits
    -- employee_id = (SELECT ...), and NULL makes that expression NULL rather
    -- than FALSE, so the row would silently vanish from every non-ScopeAll
    -- list instead of being denied. The hrm_goals.employee_id precedent.
    employee_id UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    course_id   UUID        NOT NULL REFERENCES hrm_courses(id) ON DELETE RESTRICT,
    -- THE PIN. RESTRICT, not CASCADE or SET NULL: a version with enrollments
    -- against it is history and must not be deletable, and an enrollment that
    -- lost its version could not say what the learner actually completed.
    version_id  UUID        NOT NULL REFERENCES hrm_course_versions(id) ON DELETE RESTRICT,

    status      TEXT        NOT NULL DEFAULT 'assigned'
                CHECK (status IN ('assigned', 'in_progress', 'completed', 'failed', 'cancelled')),

    -- How this enrollment came about. 'rule' is set by 6B auto-assignment.
    assigned_via TEXT       NOT NULL DEFAULT 'manual'
                 CHECK (assigned_via IN ('manual', 'self', 'rule')),

    due_date     DATE,
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    assigned_by UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One live enrollment per employee per course. Partial, so a cancelled or
-- failed attempt frees them to be re-enrolled — including onto a newer
-- version, which is the normal recertification path.
CREATE UNIQUE INDEX uq_hrm_enr_employee_course_live ON hrm_enrollments (employee_id, course_id)
    WHERE status IN ('assigned', 'in_progress');

CREATE INDEX idx_hrm_enr_employee ON hrm_enrollments (employee_id, status);
CREATE INDEX idx_hrm_enr_org_status ON hrm_enrollments (org_id, status);
CREATE INDEX idx_hrm_enr_version ON hrm_enrollments (version_id);
CREATE INDEX idx_hrm_enr_due ON hrm_enrollments (due_date)
    WHERE due_date IS NOT NULL AND status IN ('assigned', 'in_progress');

COMMENT ON COLUMN hrm_enrollments.version_id IS 'Pinned at enrollment; RESTRICT because a version with enrollments is history. An edit to the course cannot rewrite what this learner was assessed on';

-- ------------------------------------------------------------
-- hrm_lesson_progress
--
-- No org_id (reached via enrollment_id). One row per lesson per enrollment,
-- created lazily on first interaction.
-- ------------------------------------------------------------
CREATE TABLE hrm_lesson_progress (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id     TEXT        NOT NULL UNIQUE
                                  DEFAULT ('lprg_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    enrollment_id UUID        NOT NULL REFERENCES hrm_enrollments(id) ON DELETE CASCADE,
    lesson_id     UUID        NOT NULL REFERENCES hrm_course_lessons(id) ON DELETE CASCADE,

    status        TEXT        NOT NULL DEFAULT 'not_started'
                  CHECK (status IN ('not_started', 'in_progress', 'completed')),

    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_hrm_lprg_enrollment_lesson ON hrm_lesson_progress (enrollment_id, lesson_id);
CREATE INDEX idx_hrm_lprg_enrollment ON hrm_lesson_progress (enrollment_id, status);

COMMENT ON TABLE hrm_lesson_progress IS 'Per-lesson state; course completion percentage is COMPUTED from these rows and never stored (migration 00076 rule)';

-- ------------------------------------------------------------
-- hrm_quiz_attempts — a graded run at a quiz lesson.
--
-- score and passed are STORED, decided once at submit. See the header: the
-- answer key is keyed on platform_form_questions.id, and that reference goes
-- NULL if a question is deleted, so a re-grade would silently score zero.
-- ------------------------------------------------------------
CREATE TABLE hrm_quiz_attempts (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id      TEXT        NOT NULL UNIQUE
                                   DEFAULT ('qatt_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id         UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    enrollment_id  UUID        NOT NULL REFERENCES hrm_enrollments(id) ON DELETE CASCADE,
    lesson_id      UUID        NOT NULL REFERENCES hrm_course_lessons(id) ON DELETE CASCADE,

    attempt_number INTEGER     NOT NULL CHECK (attempt_number >= 1),

    -- The learner's answers live in the form engine. SET NULL so purging an
    -- instance is not blocked; the GRADE below survives regardless, which is
    -- the entire point.
    form_instance_id UUID      REFERENCES platform_form_instances(id) ON DELETE SET NULL,

    -- Frozen at grading time.
    score          NUMERIC(5,2) CHECK (score IS NULL OR (score >= 0 AND score <= 100)),
    points_earned  NUMERIC(8,2),
    points_possible NUMERIC(8,2),
    passed         BOOLEAN,
    -- Snapshotted from the lesson so a later pass_mark change cannot flip a
    -- historical result from fail to pass.
    pass_mark_snapshot NUMERIC(5,2),

    started_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    submitted_at   TIMESTAMPTZ,
    graded_at      TIMESTAMPTZ,

    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_hrm_qatt_enrollment_lesson_number
    ON hrm_quiz_attempts (enrollment_id, lesson_id, attempt_number);
CREATE INDEX idx_hrm_qatt_enrollment ON hrm_quiz_attempts (enrollment_id, lesson_id);
CREATE INDEX idx_hrm_qatt_instance ON hrm_quiz_attempts (form_instance_id)
    WHERE form_instance_id IS NOT NULL;

COMMENT ON COLUMN hrm_quiz_attempts.score IS 'Frozen at submit. Never re-derived: the answer key is keyed on a question_id that platform_form_responses nulls on delete, so a re-grade could silently score zero';
COMMENT ON COLUMN hrm_quiz_attempts.pass_mark_snapshot IS 'Frozen from the lesson so a later pass_mark change cannot flip a historical result';

-- ------------------------------------------------------------
-- hrm_quiz_answer_keys — the correct answers.
--
-- Keyed on platform_form_questions.id, but owned by HRM. platform/forms never
-- learns what "correct" means, so appraisals and 360 feedback do not carry an
-- assessment column they never read.
--
-- No query that serves a learner may join this table. That is the whole
-- mechanism — the same structural separation 5C used for 360 anonymity,
-- where the fix for "don't leak X" was to put X where the read path cannot
-- reach it, not to remember to filter it out.
-- ------------------------------------------------------------
CREATE TABLE hrm_quiz_answer_keys (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE
                                DEFAULT ('qkey_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- CASCADE: a deleted question has no correct answer to remember. Already
    -- graded attempts are unaffected because their score is stored.
    question_id UUID        NOT NULL REFERENCES platform_form_questions(id) ON DELETE CASCADE,

    -- Typed correct answer, mirroring the engine's typed answer columns.
    -- Exactly one is populated, selected by the question's type — validated
    -- in the service, not by a CHECK, because a CHECK spanning these would
    -- fire on any UPDATE (the 00084 scale_min/scale_max reasoning).
    correct_text    TEXT,
    correct_number  NUMERIC(18,4),
    correct_boolean BOOLEAN,
    correct_options TEXT[],

    -- Weight of this question within the quiz.
    points      NUMERIC(8,2) NOT NULL DEFAULT 1 CHECK (points > 0),
    -- For multi_select: whether every option must match exactly, or partial
    -- credit is awarded per correct option.
    partial_credit BOOLEAN   NOT NULL DEFAULT FALSE,

    explanation TEXT,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_hrm_qkey_question ON hrm_quiz_answer_keys (question_id);
CREATE INDEX idx_hrm_qkey_org ON hrm_quiz_answer_keys (org_id);

COMMENT ON TABLE hrm_quiz_answer_keys IS 'Correct answers, owned by HRM so platform/forms carries no assessment semantics. NO query serving a learner may join this table — that separation IS the protection';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_quiz_answer_keys;
DROP TABLE IF EXISTS hrm_quiz_attempts;
DROP TABLE IF EXISTS hrm_lesson_progress;
DROP TABLE IF EXISTS hrm_enrollments;
DROP TABLE IF EXISTS hrm_course_lessons;
DROP TABLE IF EXISTS hrm_course_modules;
DROP TABLE IF EXISTS hrm_course_versions;
DROP TABLE IF EXISTS hrm_courses;

-- +goose StatementEnd
