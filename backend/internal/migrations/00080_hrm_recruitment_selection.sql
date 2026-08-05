-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00080_hrm_recruitment_selection
--
-- Phase 4B of the HRM Extended Build Plan: Recruitment / ATS, selection &
-- close half. Five new tables plus one ALTER and three CHECK-constraint
-- alterations:
--   hrm_interviews             — scheduled interviews for an application
--   hrm_interview_panelists    — who is on the panel
--   hrm_interview_scorecards   — fixed-shape per-panelist scoring
--   hrm_offers                 — approval-gated compensation offers
--   hrm_referrals              — referral bonus-program lifecycle
--   hrm_employees.source_candidate_id — provenance for hire conversions
--
-- Design notes:
--   • hrm_interview_scorecards is a fixed-shape table, deliberately NOT a
--     mini form engine — Phase 5 builds the real form engine and names
--     interview scorecards as its consumer #1; building a bespoke one here
--     would pre-empt that primitive (the same reasoning that kept Phase 3's
--     checklist engine from being built speculatively).
--   • submitted_at is the scorecard's only status field: NULL = draft,
--     set = immutable (mirrors the finalized-payslip / leave-balance-
--     snapshot precedent — no separate "locked" boolean needed).
--   • "Interviewer cannot see others' scores before submitting their own"
--     is NOT expressible with the Phase 1 scope tiers (internal/hrm/scope's
--     Predicate/AuthorizeRecordAccess both hard-code FROM hrm_employees,
--     and every tier is state-independent). It is a bespoke rule in
--     scorecards_service.go, not a schema-level constraint.
--   • hrm_offers reuses the approval engine exactly like hrm_job_requisitions
--     did in migration 00078 — see the CHECK-constraint alterations below.
--   • hrm_referrals is distinct from hrm_candidates.referred_by_employee_id
--     (00078) — that column is lightweight "who brought this candidate in"
--     provenance kept regardless of whether a bonus applies; this table is
--     the formal bonus-program lifecycle, created explicitly.
--   • hrm_employees.source_candidate_id is the provenance the build plan
--     asks for ("candidate_id retained on the employee for provenance").
-- ============================================================

-- ------------------------------------------------------------
-- hrm_interviews
-- ------------------------------------------------------------
CREATE TABLE hrm_interviews (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id        TEXT        NOT NULL UNIQUE
                                     DEFAULT ('intv_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id           UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    application_id   UUID        NOT NULL REFERENCES hrm_applications(id) ON DELETE CASCADE,

    scheduled_at     TIMESTAMPTZ NOT NULL,
    duration_minutes INTEGER     NOT NULL DEFAULT 60 CHECK (duration_minutes > 0),
    mode             TEXT        NOT NULL DEFAULT 'video' CHECK (mode IN ('onsite', 'phone', 'video')),
    location         TEXT,
    meeting_url      TEXT,

    status           TEXT        NOT NULL DEFAULT 'scheduled'
                                     CHECK (status IN ('scheduled', 'completed', 'cancelled', 'no_show')),
    -- Recommendation signal only — does NOT auto-move the application's
    -- pipeline stage. The recruiter still calls MoveApplication explicitly.
    outcome          TEXT        CHECK (outcome IN ('advance', 'reject', 'hold')),
    notes            TEXT,

    created_by       UUID        NOT NULL REFERENCES users(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_intv_org_id         ON hrm_interviews (org_id);
CREATE INDEX idx_hrm_intv_application_id ON hrm_interviews (application_id);
CREATE INDEX idx_hrm_intv_scheduled_at   ON hrm_interviews (org_id, scheduled_at);

COMMENT ON TABLE  hrm_interviews IS 'Scheduled interviews for an application';
COMMENT ON COLUMN hrm_interviews.outcome IS 'Recommendation only — never auto-moves the pipeline stage';

-- ------------------------------------------------------------
-- hrm_interview_panelists
-- ------------------------------------------------------------
CREATE TABLE hrm_interview_panelists (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id     TEXT        NOT NULL UNIQUE
                                  DEFAULT ('panl_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    interview_id  UUID        NOT NULL REFERENCES hrm_interviews(id) ON DELETE CASCADE,
    employee_id   UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    panelist_role TEXT,       -- free text: "interviewer", "hiring manager", "note taker"
    is_lead       BOOLEAN     NOT NULL DEFAULT FALSE,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX        idx_hrm_panl_interview_id ON hrm_interview_panelists (interview_id);
CREATE INDEX        idx_hrm_panl_employee_id  ON hrm_interview_panelists (employee_id);
CREATE UNIQUE INDEX uq_hrm_panl_interview_employee ON hrm_interview_panelists (interview_id, employee_id);

COMMENT ON TABLE hrm_interview_panelists IS 'Who is on the interview panel — scorecard write access is validated against this table';

-- ------------------------------------------------------------
-- hrm_interview_scorecards
-- ------------------------------------------------------------
CREATE TABLE hrm_interview_scorecards (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id            TEXT        NOT NULL UNIQUE
                                         DEFAULT ('sc_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    -- No org_id: reached via interview_id → hrm_interviews → hrm_applications,
    -- the hrm_application_stage_history precedent.
    interview_id         UUID        NOT NULL REFERENCES hrm_interviews(id) ON DELETE CASCADE,
    panelist_employee_id UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    overall_rating       INTEGER     CHECK (overall_rating BETWEEN 1 AND 5),
    technical_score      INTEGER     CHECK (technical_score BETWEEN 1 AND 5),
    communication_score  INTEGER     CHECK (communication_score BETWEEN 1 AND 5),
    culture_fit_score    INTEGER     CHECK (culture_fit_score BETWEEN 1 AND 5),
    recommendation       TEXT        CHECK (recommendation IN ('strong_hire', 'hire', 'no_hire', 'strong_no_hire')),
    strengths            TEXT,
    concerns             TEXT,

    -- NULL = draft (visible only to the panelist who owns it); once set,
    -- the row is immutable (enforced in the service, the finalized-payslip
    -- precedent — not a DB trigger, matching how this codebase enforces
    -- "immutable once finalized" everywhere else).
    submitted_at         TIMESTAMPTZ,

    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX        idx_hrm_sc_interview_id ON hrm_interview_scorecards (interview_id);
CREATE UNIQUE INDEX uq_hrm_sc_interview_panelist ON hrm_interview_scorecards (interview_id, panelist_employee_id);

COMMENT ON TABLE  hrm_interview_scorecards IS 'Fixed-shape per-panelist scoring — deliberately not a form engine; see migration header';
COMMENT ON COLUMN hrm_interview_scorecards.submitted_at IS 'NULL = draft, visible only to its own panelist; set = immutable and visible to every panelist on this interview';

-- ------------------------------------------------------------
-- hrm_offers
-- ------------------------------------------------------------
CREATE TABLE hrm_offers (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id            TEXT        NOT NULL UNIQUE
                                         DEFAULT ('ofr_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id               UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    application_id       UUID        NOT NULL REFERENCES hrm_applications(id) ON DELETE CASCADE,
    -- Denormalized alongside application_id, mirroring hrm_applications'
    -- own pipeline_id+stage_id redundancy — lets offer validation check
    -- salary against the requisition's band without a join through the
    -- application → posting → requisition chain.
    requisition_id       UUID        NOT NULL REFERENCES hrm_job_requisitions(id) ON DELETE RESTRICT,

    base_salary          NUMERIC(15,2),
    salary_currency      CHAR(3)     NOT NULL DEFAULT 'USD',
    signing_bonus        NUMERIC(15,2),
    equity_details       TEXT,       -- free text; a real cap-table feature is out of scope
    start_date           DATE,
    expires_at           TIMESTAMPTZ,

    status               TEXT        NOT NULL DEFAULT 'draft'
                                         CHECK (status IN ('draft', 'pending_approval', 'approved', 'rejected',
                                                           'sent', 'accepted', 'declined', 'rescinded', 'expired')),
    approval_instance_id UUID        REFERENCES hrm_approval_instances(id) ON DELETE SET NULL,
    -- Optional link to the generated/uploaded offer letter. No auto-
    -- generation exists anywhere in this codebase (doctemplates is
    -- preview-only, browser renders to PDF) — this is populated by a
    -- separate, existing hrm_employee_documents write if the caller wants one.
    document_id          UUID        REFERENCES hrm_employee_documents(id) ON DELETE SET NULL,

    created_by           UUID        NOT NULL REFERENCES users(id),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_ofr_org_id         ON hrm_offers (org_id);
CREATE INDEX idx_hrm_ofr_application_id ON hrm_offers (application_id);
CREATE INDEX idx_hrm_ofr_status         ON hrm_offers (org_id, status);

COMMENT ON TABLE  hrm_offers IS 'Approval-gated compensation offers; reuses the hrm_approval_* engine via action_type/entity_type = offer';

-- ------------------------------------------------------------
-- hrm_referrals
-- ------------------------------------------------------------
CREATE TABLE hrm_referrals (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id               TEXT        NOT NULL UNIQUE
                                            DEFAULT ('ref_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                  UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    candidate_id            UUID        NOT NULL REFERENCES hrm_candidates(id) ON DELETE CASCADE,
    referred_by_employee_id UUID        REFERENCES hrm_employees(id) ON DELETE SET NULL,
    application_id          UUID        REFERENCES hrm_applications(id) ON DELETE SET NULL,

    status                  TEXT        NOT NULL DEFAULT 'submitted'
                                            CHECK (status IN ('submitted', 'candidate_hired', 'bonus_pending',
                                                              'bonus_paid', 'not_eligible')),
    bonus_amount            NUMERIC(15,2),
    bonus_currency          CHAR(3)     NOT NULL DEFAULT 'USD',
    paid_at                 TIMESTAMPTZ,
    notes                   TEXT,

    created_by              UUID        NOT NULL REFERENCES users(id),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_ref_org_id       ON hrm_referrals (org_id);
CREATE INDEX idx_hrm_ref_candidate_id ON hrm_referrals (candidate_id);
CREATE INDEX idx_hrm_ref_referrer     ON hrm_referrals (referred_by_employee_id) WHERE referred_by_employee_id IS NOT NULL;
CREATE INDEX idx_hrm_ref_status       ON hrm_referrals (org_id, status);

COMMENT ON TABLE hrm_referrals IS 'Referral bonus-program lifecycle — distinct from hrm_candidates.referred_by_employee_id, which is lightweight provenance kept regardless of a formal bonus';

-- ------------------------------------------------------------
-- hrm_employees.source_candidate_id
-- ------------------------------------------------------------
ALTER TABLE hrm_employees
    ADD COLUMN source_candidate_id UUID REFERENCES hrm_candidates(id) ON DELETE SET NULL;

CREATE INDEX idx_hrm_emp_source_candidate ON hrm_employees (source_candidate_id) WHERE source_candidate_id IS NOT NULL;

COMMENT ON COLUMN hrm_employees.source_candidate_id IS 'Provenance for hire-converted employees, set by recruitment.HireApplication; NULL for ordinary HR-created employees';

-- ------------------------------------------------------------
-- Approval engine + document templates: register 'offer'.
--
-- Same two-constraint alteration pattern as 00049 ('award') and 00078
-- ('job_requisition'). Also adds 'offer' to hrm_employee_documents.
-- related_type, which currently has no such value — 'offer' is a
-- first-class concept here the same way 'award'/'job_requisition' were,
-- not a one-off that belongs behind the 'custom' escape hatch.
-- ------------------------------------------------------------
ALTER TABLE hrm_approval_templates
    DROP CONSTRAINT IF EXISTS hrm_approval_templates_action_type_check,
    ADD CONSTRAINT hrm_approval_templates_action_type_check
        CHECK (action_type IN (
            'leave', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'offer', 'custom'
        ));

ALTER TABLE hrm_approval_instances
    DROP CONSTRAINT IF EXISTS hrm_approval_instances_entity_type_check,
    ADD CONSTRAINT hrm_approval_instances_entity_type_check
        CHECK (entity_type IN (
            'leave_request', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'offer', 'custom'
        ));

ALTER TABLE hrm_employee_documents
    DROP CONSTRAINT IF EXISTS hrm_employee_documents_related_type_check,
    ADD CONSTRAINT hrm_employee_documents_related_type_check
        CHECK (related_type IN (
            'warning', 'promotion', 'transfer', 'resignation', 'termination', 'offer', 'custom'
        ));

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE hrm_employee_documents
    DROP CONSTRAINT IF EXISTS hrm_employee_documents_related_type_check,
    ADD CONSTRAINT hrm_employee_documents_related_type_check
        CHECK (related_type IN (
            'warning', 'promotion', 'transfer', 'resignation', 'termination', 'custom'
        ));

ALTER TABLE hrm_approval_instances
    DROP CONSTRAINT IF EXISTS hrm_approval_instances_entity_type_check,
    ADD CONSTRAINT hrm_approval_instances_entity_type_check
        CHECK (entity_type IN (
            'leave_request', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'custom'
        ));

ALTER TABLE hrm_approval_templates
    DROP CONSTRAINT IF EXISTS hrm_approval_templates_action_type_check,
    ADD CONSTRAINT hrm_approval_templates_action_type_check
        CHECK (action_type IN (
            'leave', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'job_requisition', 'custom'
        ));

DROP INDEX IF EXISTS idx_hrm_emp_source_candidate;
ALTER TABLE hrm_employees DROP COLUMN IF EXISTS source_candidate_id;

DROP TABLE IF EXISTS hrm_referrals;
DROP TABLE IF EXISTS hrm_offers;
DROP TABLE IF EXISTS hrm_interview_scorecards;
DROP TABLE IF EXISTS hrm_interview_panelists;
DROP TABLE IF EXISTS hrm_interviews;

-- +goose StatementEnd
