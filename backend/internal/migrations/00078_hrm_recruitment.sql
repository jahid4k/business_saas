-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00078_hrm_recruitment
--
-- Phase 4A of the HRM Extended Build Plan: Recruitment / ATS,
-- intake & pipeline half. Seven tables:
--   hrm_recruitment_pipelines      — configurable hiring pipelines
--   hrm_recruitment_stages         — ordered stages within a pipeline
--   hrm_job_requisitions           — approval-gated headcount requests
--   hrm_job_postings               — the advertised role (carries public_slug)
--   hrm_candidates                 — a person
--   hrm_applications               — a person applying to a posting
--   hrm_application_stage_history  — append-only pipeline movement log
--
-- Phase 4B adds interviews, panelists, scorecards, offers, referrals and
-- the hire → employee conversion. Columns those need that cost nothing
-- now are included here so 4B requires no ALTER:
-- hrm_applications.converted_employee_id and
-- hrm_candidates.referred_by_employee_id.
--
-- Design notes:
--   • Pipelines/stages mirror crm_pipelines/crm_pipeline_stages (00015)
--     including the denormalized org_id on the stage row — but WITH the
--     partial unique index CRM lacks. In CRM nothing enforces one default
--     per org: the column exists and is sorted on, but UpdatePipeline sets
--     it verbatim and there is no index, so an org can silently end up with
--     two or zero defaults.
--   • stage_kind is an addition over the CRM shape. crm_pipeline_stages has
--     no terminal/won/lost marker (crm_deals.status carries that instead),
--     which works for deals but not here: Phase 4B's hire conversion needs
--     to know which stage MEANS hired, and rejection needs a trigger point.
--   • hrm_application_stage_history is in this FIRST migration, deliberately.
--     crm_deals never got an equivalent — a stage change there overwrites
--     stage_id in place and the previous stage is lost forever, which is
--     precisely why deal-velocity reporting is impossible today. Do not
--     repeat that here.
--
-- ⚠ Two constraint choices that look like mistakes and are not:
--
--   1. hrm_applications.created_by is NULLABLE. crm_leads.created_by is
--      NOT NULL, and that is the direct cause of a live bug (Capture Fix
--      Pass A item 1): system-generated leads pass "" and fail on an
--      invalid-UUID error. When the public /pub/careers apply endpoint
--      lands there is no authenticated user to attribute. Nullable now
--      means no migration and no bug later.
--
--   2. hrm_candidates has a REAL unique index on (org_id, LOWER(email)).
--      crm_leads has no index on email at all — its dedup is app-level,
--      case-sensitive (`email = $2`), and has a check-then-insert race with
--      no constraint to catch it. Bob@x.com and bob@x.com are two leads
--      there. They are one candidate here.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_recruitment_pipelines
-- ------------------------------------------------------------
CREATE TABLE hrm_recruitment_pipelines (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE
                                DEFAULT ('rpipe_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name        TEXT        NOT NULL,
    description TEXT,
    is_default  BOOLEAN     NOT NULL DEFAULT FALSE,
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,

    created_by  UUID        NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX        idx_hrm_rpipe_org_id   ON hrm_recruitment_pipelines (org_id);
CREATE UNIQUE INDEX uq_hrm_rpipe_org_name  ON hrm_recruitment_pipelines (org_id, LOWER(name));
-- The guard crm_pipelines never got. The service clears the prior default
-- inside the same transaction that sets the new one (the
-- platform_checklist_templates pattern), so this never fires in normal use.
CREATE UNIQUE INDEX uq_hrm_rpipe_default   ON hrm_recruitment_pipelines (org_id)
    WHERE is_default = TRUE AND is_active = TRUE;

COMMENT ON TABLE  hrm_recruitment_pipelines IS 'Configurable hiring pipelines; mirrors crm_pipelines but enforces one default per org';
COMMENT ON COLUMN hrm_recruitment_pipelines.is_default IS 'At most one default per org, enforced by uq_hrm_rpipe_default';

-- ------------------------------------------------------------
-- hrm_recruitment_stages
-- ------------------------------------------------------------
CREATE TABLE hrm_recruitment_stages (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE
                                DEFAULT ('rstg_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    -- Denormalized alongside pipeline_id, matching crm_pipeline_stages, so
    -- stage queries can filter org_id without a join.
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    pipeline_id UUID        NOT NULL REFERENCES hrm_recruitment_pipelines(id) ON DELETE CASCADE,

    name        TEXT        NOT NULL,
    -- Rewritten 0..n-1 from array index on reorder; no unique index, since a
    -- drag-reorder would 23505 against one mid-transaction.
    position    INTEGER     NOT NULL DEFAULT 0,
    stage_kind  TEXT        NOT NULL DEFAULT 'in_progress'
                                CHECK (stage_kind IN ('applied', 'in_progress', 'offer', 'hired', 'rejected')),

    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX        idx_hrm_rstg_pipeline_id  ON hrm_recruitment_stages (pipeline_id);
CREATE INDEX        idx_hrm_rstg_org_id       ON hrm_recruitment_stages (org_id);
CREATE UNIQUE INDEX uq_hrm_rstg_pipeline_name ON hrm_recruitment_stages (pipeline_id, LOWER(name));

COMMENT ON TABLE  hrm_recruitment_stages IS 'Ordered stages within a hiring pipeline';
COMMENT ON COLUMN hrm_recruitment_stages.stage_kind IS 'Semantic marker crm_pipeline_stages lacks; hired/rejected are terminal and drive application status';

-- ------------------------------------------------------------
-- hrm_job_requisitions
-- ------------------------------------------------------------
CREATE TABLE hrm_job_requisitions (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id            TEXT        NOT NULL UNIQUE
                                         DEFAULT ('jrq_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id               UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    title                TEXT        NOT NULL,
    department_id        UUID        REFERENCES hrm_departments(id) ON DELETE SET NULL,
    position_id          UUID        REFERENCES hrm_positions(id)   ON DELETE SET NULL,
    hiring_manager_id    UUID        REFERENCES hrm_employees(id)   ON DELETE SET NULL,

    employment_type      TEXT        NOT NULL DEFAULT 'full_time'
                                         CHECK (employment_type IN ('full_time', 'part_time', 'contractor', 'intern')),
    openings             INTEGER     NOT NULL DEFAULT 1 CHECK (openings > 0),
    -- Maintained by Phase 4B's hire conversion; always 0 in 4A.
    filled_count         INTEGER     NOT NULL DEFAULT 0 CHECK (filled_count >= 0),

    location             TEXT,
    salary_min           NUMERIC(15,2),
    salary_max           NUMERIC(15,2),
    salary_currency      CHAR(3)     NOT NULL DEFAULT 'USD',

    justification        TEXT,
    target_start_date    DATE,

    status               TEXT        NOT NULL DEFAULT 'draft'
                                         CHECK (status IN ('draft', 'pending_approval', 'approved',
                                                           'rejected', 'on_hold', 'closed', 'cancelled')),
    approval_instance_id UUID        REFERENCES hrm_approval_instances(id) ON DELETE SET NULL,
    closed_at            TIMESTAMPTZ,
    close_reason         TEXT,

    created_by           UUID        NOT NULL REFERENCES users(id),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_jrq_salary_range
        CHECK (salary_min IS NULL OR salary_max IS NULL OR salary_max >= salary_min)
);

CREATE INDEX idx_hrm_jrq_org_id        ON hrm_job_requisitions (org_id);
CREATE INDEX idx_hrm_jrq_status        ON hrm_job_requisitions (org_id, status);
CREATE INDEX idx_hrm_jrq_department_id ON hrm_job_requisitions (department_id) WHERE department_id IS NOT NULL;
CREATE INDEX idx_hrm_jrq_hiring_mgr    ON hrm_job_requisitions (hiring_manager_id) WHERE hiring_manager_id IS NOT NULL;

COMMENT ON TABLE  hrm_job_requisitions IS 'Approval-gated headcount requests; reuses the hrm_approval_* engine via action_type/entity_type = job_requisition';
COMMENT ON COLUMN hrm_job_requisitions.filled_count IS 'Incremented by Phase 4B hire conversion; always 0 until that ships';

-- ------------------------------------------------------------
-- hrm_job_postings
-- ------------------------------------------------------------
CREATE TABLE hrm_job_postings (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id            TEXT        NOT NULL UNIQUE
                                         DEFAULT ('jpst_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id               UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    requisition_id       UUID        NOT NULL REFERENCES hrm_job_requisitions(id) ON DELETE CASCADE,
    -- RESTRICT, not CASCADE: deleting a pipeline that live postings route
    -- applications through must fail loudly, not silently orphan them.
    pipeline_id          UUID        NOT NULL REFERENCES hrm_recruitment_pipelines(id) ON DELETE RESTRICT,

    title                TEXT        NOT NULL,
    description_markdown TEXT        NOT NULL DEFAULT '',
    -- Written now even though Phase 4A has no public route: the slug belongs
    -- to the posting regardless of who reads it, and the eventual public URL
    -- is /pub/careers/:orgSlug/:postingSlug, hence per-org uniqueness.
    public_slug          TEXT        NOT NULL,

    location             TEXT,
    is_remote            BOOLEAN     NOT NULL DEFAULT FALSE,
    employment_type      TEXT        NOT NULL DEFAULT 'full_time'
                                         CHECK (employment_type IN ('full_time', 'part_time', 'contractor', 'intern')),

    status               TEXT        NOT NULL DEFAULT 'draft'
                                         CHECK (status IN ('draft', 'published', 'closed')),
    published_at         TIMESTAMPTZ,
    closed_at            TIMESTAMPTZ,

    created_by           UUID        NOT NULL REFERENCES users(id),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX        idx_hrm_jpst_org_id         ON hrm_job_postings (org_id);
CREATE INDEX        idx_hrm_jpst_requisition_id ON hrm_job_postings (requisition_id);
CREATE INDEX        idx_hrm_jpst_status         ON hrm_job_postings (org_id, status);
CREATE UNIQUE INDEX uq_hrm_jpst_org_slug        ON hrm_job_postings (org_id, LOWER(public_slug));

COMMENT ON TABLE  hrm_job_postings IS 'The advertised role. public_slug is written now; the public careers page that reads it is a later phase';
COMMENT ON COLUMN hrm_job_postings.public_slug IS 'Unique per org; the eventual public URL is /pub/careers/:orgSlug/:postingSlug';

-- ------------------------------------------------------------
-- hrm_candidates
-- ------------------------------------------------------------
CREATE TABLE hrm_candidates (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id               TEXT        NOT NULL UNIQUE
                                            DEFAULT ('cand_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                  UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    first_name              TEXT        NOT NULL,
    last_name               TEXT,
    email                   TEXT,
    phone                   TEXT,
    headline                TEXT,
    location                TEXT,
    linkedin_url            TEXT,
    portfolio_url           TEXT,

    source                  TEXT        NOT NULL DEFAULT 'direct'
                                            CHECK (source IN ('careers_page', 'referral', 'agency',
                                                              'sourced', 'direct', 'import', 'other')),
    -- Phase 4B referrals read this; free to add now, an ALTER later.
    referred_by_employee_id UUID        REFERENCES hrm_employees(id) ON DELETE SET NULL,

    -- Resume files are content-addressed by sha256 and stored OUTSIDE
    -- ./uploads (which is served unauthenticated by a static handler).
    -- Two candidates with the identical file share one path on disk, so
    -- unlinking on delete must be reference-counted, never naive.
    resume_file_path        TEXT,
    resume_file_name        TEXT,
    resume_mime_type        TEXT,
    resume_size_bytes       BIGINT,
    resume_sha256           TEXT,

    notes                   TEXT,
    -- Written for a future GDPR purge job; read by nothing yet. The column
    -- exists now so that job needs no migration (the Phase 3 due_date precedent).
    purge_after             DATE,

    created_by              UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at              TIMESTAMPTZ
);

CREATE INDEX idx_hrm_cand_org_id     ON hrm_candidates (org_id);
CREATE INDEX idx_hrm_cand_deleted_at ON hrm_candidates (deleted_at);
CREATE INDEX idx_hrm_cand_referrer   ON hrm_candidates (referred_by_employee_id) WHERE referred_by_employee_id IS NOT NULL;
CREATE INDEX idx_hrm_cand_sha256     ON hrm_candidates (resume_sha256) WHERE resume_sha256 IS NOT NULL;
-- The real constraint crm_leads never got. Case-insensitive, soft-delete
-- aware, and enforced by the database rather than a racy app-level check.
CREATE UNIQUE INDEX uq_hrm_cand_org_email ON hrm_candidates (org_id, LOWER(email))
    WHERE email IS NOT NULL AND deleted_at IS NULL;

COMMENT ON TABLE  hrm_candidates IS 'A person. Distinct from hrm_applications — one candidate may apply to many postings';
COMMENT ON COLUMN hrm_candidates.resume_file_path IS 'Content-addressed path outside ./uploads; shared between candidates with identical files, so deletion must be reference-counted';
COMMENT ON COLUMN hrm_candidates.purge_after IS 'Deliberately a dead column until a GDPR purge job is built';

-- ------------------------------------------------------------
-- hrm_applications
-- ------------------------------------------------------------
CREATE TABLE hrm_applications (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id             TEXT        NOT NULL UNIQUE
                                          DEFAULT ('appl_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    candidate_id          UUID        NOT NULL REFERENCES hrm_candidates(id) ON DELETE CASCADE,
    posting_id            UUID        NOT NULL REFERENCES hrm_job_postings(id) ON DELETE RESTRICT,
    -- pipeline_id is stored alongside stage_id, mirroring crm_deals. The
    -- redundancy is load-bearing: it is what lets a stage move validate that
    -- the target stage belongs to this application's pipeline.
    pipeline_id           UUID        NOT NULL REFERENCES hrm_recruitment_pipelines(id) ON DELETE RESTRICT,
    stage_id              UUID        NOT NULL REFERENCES hrm_recruitment_stages(id) ON DELETE RESTRICT,

    status                TEXT        NOT NULL DEFAULT 'active'
                                          CHECK (status IN ('active', 'hired', 'rejected', 'withdrawn')),
    rejection_reason      TEXT,
    rejected_at           TIMESTAMPTZ,
    withdrawn_at          TIMESTAMPTZ,
    hired_at              TIMESTAMPTZ,
    -- Phase 4B hire conversion writes this for provenance.
    converted_employee_id UUID        REFERENCES hrm_employees(id) ON DELETE SET NULL,

    cover_letter          TEXT,
    source                TEXT,
    applied_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- NULLABLE deliberately — see the migration header. A public application
    -- has no authenticated user to attribute, and crm_leads' NOT NULL on the
    -- same column is a live bug.
    created_by            UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_appl_rejected CHECK (status <> 'rejected' OR rejected_at IS NOT NULL),
    CONSTRAINT chk_hrm_appl_hired    CHECK (status <> 'hired'    OR hired_at    IS NOT NULL)
);

CREATE INDEX idx_hrm_appl_org_id       ON hrm_applications (org_id);
CREATE INDEX idx_hrm_appl_candidate_id ON hrm_applications (candidate_id);
CREATE INDEX idx_hrm_appl_posting_id   ON hrm_applications (posting_id);
CREATE INDEX idx_hrm_appl_stage_id     ON hrm_applications (stage_id);
CREATE INDEX idx_hrm_appl_status       ON hrm_applications (org_id, status);
-- One live application per candidate per posting. Withdrawn ones are excluded
-- so a candidate who withdrew may re-apply to the same role.
CREATE UNIQUE INDEX uq_hrm_appl_candidate_posting
    ON hrm_applications (candidate_id, posting_id) WHERE status <> 'withdrawn';

COMMENT ON TABLE  hrm_applications IS 'A candidate applying to a posting. Stage lives HERE, not on the candidate';
COMMENT ON COLUMN hrm_applications.created_by IS 'Nullable by design — a public application has no authenticated actor (see migration header)';

-- ------------------------------------------------------------
-- hrm_application_stage_history
-- ------------------------------------------------------------
CREATE TABLE hrm_application_stage_history (
    id                        UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id            UUID        NOT NULL REFERENCES hrm_applications(id) ON DELETE CASCADE,

    from_stage_id             UUID        REFERENCES hrm_recruitment_stages(id) ON DELETE SET NULL,
    to_stage_id               UUID        REFERENCES hrm_recruitment_stages(id) ON DELETE SET NULL,
    -- Snapshot names so a renamed or deleted stage does not erase history.
    from_stage_name           TEXT,
    to_stage_name             TEXT        NOT NULL,

    moved_by                  UUID        REFERENCES users(id) ON DELETE SET NULL,
    moved_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- The field that makes pipeline-velocity reporting possible. Computed at
    -- write time inside the same transaction as the move. NULL on the initial
    -- placement row (there is no previous stage to measure).
    seconds_in_previous_stage BIGINT,
    note                      TEXT
);

-- No org_id: reached via the parent application, the hrm_approval_decisions
-- precedent. Append-only — no updated_at, and nothing ever UPDATEs this table.
CREATE INDEX idx_hrm_ash_application_id ON hrm_application_stage_history (application_id, moved_at);

COMMENT ON TABLE  hrm_application_stage_history IS 'Append-only pipeline movement log. Exists in the first migration precisely because crm_deals never got one and deal velocity is unrecoverable as a result';
COMMENT ON COLUMN hrm_application_stage_history.seconds_in_previous_stage IS 'NULL on initial placement; otherwise moved_at minus the prior transition, computed in the same transaction as the move';

-- ------------------------------------------------------------
-- Approval engine: register the job_requisition action/entity type.
--
-- BOTH constraints must be altered together. Migration 00049 exists because
-- 'award' was added in Go without this step and every award approval failed
-- at the database layer while the code compiled cleanly. Same pattern here.
-- ------------------------------------------------------------
ALTER TABLE hrm_approval_templates
    DROP CONSTRAINT IF EXISTS hrm_approval_templates_action_type_check,
    ADD CONSTRAINT hrm_approval_templates_action_type_check
        CHECK (action_type IN (
            'leave', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'custom'
        ));

ALTER TABLE hrm_approval_instances
    DROP CONSTRAINT IF EXISTS hrm_approval_instances_entity_type_check,
    ADD CONSTRAINT hrm_approval_instances_entity_type_check
        CHECK (entity_type IN (
            'leave_request', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'custom'
        ));

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE hrm_approval_templates
    DROP CONSTRAINT IF EXISTS hrm_approval_templates_action_type_check,
    ADD CONSTRAINT hrm_approval_templates_action_type_check
        CHECK (action_type IN (
            'leave', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'custom'
        ));

ALTER TABLE hrm_approval_instances
    DROP CONSTRAINT IF EXISTS hrm_approval_instances_entity_type_check,
    ADD CONSTRAINT hrm_approval_instances_entity_type_check
        CHECK (entity_type IN (
            'leave_request', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'custom'
        ));

DROP TABLE IF EXISTS hrm_application_stage_history;
DROP TABLE IF EXISTS hrm_applications;
DROP TABLE IF EXISTS hrm_candidates;
DROP TABLE IF EXISTS hrm_job_postings;
DROP TABLE IF EXISTS hrm_job_requisitions;
DROP TABLE IF EXISTS hrm_recruitment_stages;
DROP TABLE IF EXISTS hrm_recruitment_pipelines;

-- +goose StatementEnd
