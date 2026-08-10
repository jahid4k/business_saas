-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00084_platform_forms
--
-- Phase 5B of the HRM Extended Build Plan, part 1 of 2: the form engine
-- platform primitive. Five tables:
--   platform_form_templates  — the authored definition
--   platform_form_sections   — grouping within a template
--   platform_form_questions  — typed questions
--   platform_form_instances  — one filled-in copy, snapshotted
--   platform_form_responses  — one row per question per instance
--
-- The build plan (line 260-262) specifies: "sections → typed questions →
-- typed responses → scoring → aggregate. Definition snapshotted onto each
-- instance so historical records render as authored. Responses are rows,
-- never a JSONB blob (aggregate queries need them)."
--
-- Built now, and not earlier, because its first real consumer arrives with
-- it: Phase 5B's appraisal cycles, in the same changeset. That is the
-- Phase 3 shape, where 00076's checklist engine shipped alongside HRM
-- onboarding rather than ahead of it. The build plan's own rule 1 —
-- "nothing speculative; a primitive gets built when its first real consumer
-- is queued next" — is why Goals/OKR (Phase 5A, migration 00082) went first
-- despite the plan's prose listing this engine before it.
--
-- Design notes:
--
--   • A response row exists for EVERY question from instantiation, answered
--     or not — the platform_checklist_instance_items precedent, where an
--     item row is created per template item and later completed. This is
--     what makes the snapshot complete: a form renders exactly as authored
--     even if the template is edited or deleted afterwards, and unanswered
--     questions are visible rather than merely absent. Answering is an
--     UPDATE, never an INSERT.
--
--   • Responses carry the question snapshot AS COLUMNS (question_text,
--     question_type, section_title, display_order), so there is no separate
--     "instance questions" table. 00076 states the governing rule: JSONB is
--     for an opaque ordered config read as a whole
--     (hrm_approval_instances.instance_snapshot); real columns are required
--     whenever rows are individually mutated or aggregated. Responses are
--     both.
--
--   • Question OPTIONS are the one JSONB field here, and that is the same
--     rule applied honestly rather than an exception to it: an option list
--     is an opaque ordered config read as a whole, never aggregated across
--     rows. Selected VALUES land in typed response columns, which is what
--     scoring reads.
--
--   • Answers are typed columns (answer_text / answer_number /
--     answer_boolean / answer_date / answer_options), not one stringly
--     column. Scoring sums answer_number; a TEXT column holding "4" would
--     make that a cast-and-hope aggregate.
--
--   • subject_* is polymorphic with NO foreign key — the
--     platform_checklist_instances and hrm_acknowledgements precedent. It is
--     what keeps this package free of any hrm_* dependency. respondent_user_id
--     is separate and deliberate: the SUBJECT is who the form is about, the
--     RESPONDENT is who fills it in. They differ for every known consumer —
--     appraisals (subject employee, respondent self then manager), 360
--     feedback (subject employee, respondent peer), interview scorecards
--     (subject candidate, respondent panelist).
--
--   • submitted_at is the instance's only status field beyond `status`
--     itself: NULL = draft and editable, set = immutable. The
--     hrm_interview_scorecards (00080) and finalized-payslip precedent.
--
-- ⚠ There is deliberately NO generic "instantiate" or "submit" HTTP route in
-- the Go layer. A generic route would have to trust a client-supplied
-- subject_id and respondent_user_id, which is an impersonation vector.
-- Instantiation is reachable only through module-owned endpoints that
-- resolve those from their own domain — the reasoning 00076 records for
-- checklists, and it applies here with more force because a form response is
-- attributable evidence about a person.
-- ============================================================

-- ------------------------------------------------------------
-- platform_form_templates
-- ------------------------------------------------------------
CREATE TABLE platform_form_templates (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id    TEXT        NOT NULL UNIQUE
                                 DEFAULT ('fmt_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    -- org_id NOT NULL: no global/system-owned templates, the 00076 decision.
    -- A global template could not express org-specific rating language.
    org_id       UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name         TEXT        NOT NULL,
    description  TEXT,
    -- One engine with a discriminator, not one engine per consumer — the
    -- checklist_type precedent. All known consumers are seeded now; only
    -- 'appraisal' has a consumer in this phase.
    form_type    TEXT        NOT NULL
                                 CHECK (form_type IN ('appraisal', 'feedback_360', 'survey',
                                                      'assessment', 'exit_interview', 'custom')),
    is_default   BOOLEAN     NOT NULL DEFAULT FALSE,
    is_active    BOOLEAN     NOT NULL DEFAULT TRUE,

    created_by   UUID        NOT NULL REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pfrm_tpl_org_id ON platform_form_templates (org_id);
CREATE INDEX idx_pfrm_tpl_type   ON platform_form_templates (org_id, form_type);
-- Partial unique default per (org, form_type) — the guard crm_pipelines
-- lacks and 00076/00078 both added deliberately.
CREATE UNIQUE INDEX uq_pfrm_tpl_default ON platform_form_templates (org_id, form_type)
    WHERE is_default AND is_active;

COMMENT ON TABLE platform_form_templates IS 'Authored form definitions; one engine discriminated by form_type, not one engine per consumer';

-- ------------------------------------------------------------
-- platform_form_sections
-- ------------------------------------------------------------
CREATE TABLE platform_form_sections (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id     TEXT        NOT NULL UNIQUE
                                  DEFAULT ('fsec_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    template_id   UUID        NOT NULL REFERENCES platform_form_templates(id) ON DELETE CASCADE,

    title         TEXT        NOT NULL,
    description   TEXT,
    -- No unique index on display_order: a drag-reorder rewrites the whole
    -- set and would 23505 mid-flight (the 00076 note on the same column).
    display_order INTEGER     NOT NULL DEFAULT 0,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pfrm_sec_template ON platform_form_sections (template_id, display_order);

-- ------------------------------------------------------------
-- platform_form_questions
-- ------------------------------------------------------------
CREATE TABLE platform_form_questions (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id     TEXT        NOT NULL UNIQUE
                                  DEFAULT ('fqst_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    section_id    UUID        NOT NULL REFERENCES platform_form_sections(id) ON DELETE CASCADE,

    question_text TEXT        NOT NULL,
    help_text     TEXT,
    question_type TEXT        NOT NULL
                                  CHECK (question_type IN ('text', 'textarea', 'number', 'scale',
                                                           'single_select', 'multi_select',
                                                           'boolean', 'date')),
    is_required   BOOLEAN     NOT NULL DEFAULT FALSE,
    display_order INTEGER     NOT NULL DEFAULT 0,

    -- Scale bounds, meaningful only for question_type = 'scale'. Left
    -- unconstrained against question_type on purpose: a CHECK pairing the
    -- two would fire on any UPDATE that changes type, and the service
    -- validates it instead (the 00076 CHECK-versus-UPDATE reasoning).
    scale_min     INTEGER,
    scale_max     INTEGER,

    -- Ordered option list for the select types. JSONB because it is an
    -- opaque config read as a whole and never aggregated across rows — the
    -- same rule that makes RESPONSES real columns, applied to a value that
    -- genuinely is a blob. Shape: [{"value":"a","label":"Excellent"}, ...]
    options       JSONB       NOT NULL DEFAULT '[]',

    -- Scoring weight. NULL means the question does not score, which is what
    -- lets a free-text question sit alongside scored ones without diluting
    -- the total.
    weight        NUMERIC(6,2) CHECK (weight IS NULL OR weight >= 0),

    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pfrm_qst_section ON platform_form_questions (section_id, display_order);

COMMENT ON COLUMN platform_form_questions.options IS 'Ordered option list for select types; JSONB because it is read as a whole and never aggregated — unlike responses, which are rows';
COMMENT ON COLUMN platform_form_questions.weight IS 'NULL = unscored, so free-text questions do not dilute a scored form';

-- ------------------------------------------------------------
-- platform_form_instances
-- ------------------------------------------------------------
CREATE TABLE platform_form_instances (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id          TEXT        NOT NULL UNIQUE
                                       DEFAULT ('fins_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id             UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    -- Provenance only, never joined for display — the snapshot below is what
    -- renders. ON DELETE SET NULL so deleting a template cannot kill a live
    -- instance.
    template_id        UUID        REFERENCES platform_form_templates(id) ON DELETE SET NULL,

    -- Snapshot of the template at instantiation.
    template_name      TEXT        NOT NULL,
    form_type          TEXT        NOT NULL,

    -- Who the form is ABOUT. Polymorphic, no FK — see the migration header.
    subject_type       TEXT        NOT NULL CHECK (subject_type IN ('employee', 'candidate')),
    subject_id         UUID        NOT NULL,
    subject_label      TEXT        NOT NULL,   -- caller-asserted; never dereferenced

    -- Who FILLS IT IN. Distinct from the subject for every known consumer.
    respondent_user_id UUID        REFERENCES users(id) ON DELETE SET NULL,
    -- Free-text role of the respondent relative to the subject ("self",
    -- "manager", "peer", "panelist"). Not a CHECK: the vocabulary belongs to
    -- each consumer, not to the engine.
    respondent_role    TEXT,

    status             TEXT        NOT NULL DEFAULT 'draft'
                                       CHECK (status IN ('draft', 'submitted', 'cancelled')),
    -- NULL = draft and editable; set = immutable. The sole immutability
    -- signal, mirroring hrm_interview_scorecards.submitted_at (00080).
    submitted_at       TIMESTAMPTZ,

    created_by         UUID        NOT NULL REFERENCES users(id),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pfrm_ins_org_id     ON platform_form_instances (org_id);
CREATE INDEX idx_pfrm_ins_subject    ON platform_form_instances (org_id, subject_type, subject_id);
CREATE INDEX idx_pfrm_ins_respondent ON platform_form_instances (respondent_user_id)
    WHERE respondent_user_id IS NOT NULL;
CREATE INDEX idx_pfrm_ins_status     ON platform_form_instances (org_id, status);

COMMENT ON COLUMN platform_form_instances.subject_id IS 'Polymorphic, deliberately no FK — this is what keeps platform/forms free of any hrm_* dependency';
COMMENT ON COLUMN platform_form_instances.respondent_user_id IS 'Who fills the form in, as opposed to who it is about; they differ for every known consumer';
COMMENT ON COLUMN platform_form_instances.submitted_at IS 'NULL = draft and editable; set = immutable';

-- ------------------------------------------------------------
-- platform_form_responses
-- ------------------------------------------------------------
CREATE TABLE platform_form_responses (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id      TEXT        NOT NULL UNIQUE
                                   DEFAULT ('frsp_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    -- No org_id: reached via instance_id → platform_form_instances, the
    -- platform_checklist_instance_items precedent.
    instance_id    UUID        NOT NULL REFERENCES platform_form_instances(id) ON DELETE CASCADE,
    -- Provenance only, never joined for display.
    question_id    UUID        REFERENCES platform_form_questions(id) ON DELETE SET NULL,

    -- Question snapshot, frozen at instantiation. This is why there is no
    -- separate instance-questions table: the response row IS the snapshot.
    section_title  TEXT        NOT NULL,
    question_text  TEXT        NOT NULL,
    question_type  TEXT        NOT NULL,
    is_required    BOOLEAN     NOT NULL DEFAULT FALSE,
    display_order  INTEGER     NOT NULL DEFAULT 0,
    scale_min      INTEGER,
    scale_max      INTEGER,
    options        JSONB       NOT NULL DEFAULT '[]',
    weight         NUMERIC(6,2),

    -- Typed answers. Exactly one is populated, selected by question_type.
    -- Typed rather than one stringly column because scoring sums
    -- answer_number, and a TEXT column holding '4' makes that a
    -- cast-and-hope aggregate.
    answer_text    TEXT,
    answer_number  NUMERIC(18,4),
    answer_boolean BOOLEAN,
    answer_date    DATE,
    answer_options TEXT[],

    answered_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pfrm_rsp_instance ON platform_form_responses (instance_id, display_order);
-- One response row per question per instance, created at instantiation.
CREATE UNIQUE INDEX uq_pfrm_rsp_instance_question ON platform_form_responses (instance_id, question_id)
    WHERE question_id IS NOT NULL;

COMMENT ON TABLE platform_form_responses IS 'One row per question per instance, created at instantiation and later answered by UPDATE — carries both the frozen question snapshot and the typed answer';
COMMENT ON COLUMN platform_form_responses.answer_number IS 'The scoring column; scale/number/rating answers land here so aggregates need no casts';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS platform_form_responses;
DROP TABLE IF EXISTS platform_form_instances;
DROP TABLE IF EXISTS platform_form_questions;
DROP TABLE IF EXISTS platform_form_sections;
DROP TABLE IF EXISTS platform_form_templates;

-- +goose StatementEnd
