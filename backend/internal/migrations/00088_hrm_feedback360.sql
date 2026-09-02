-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00088_hrm_feedback360
--
-- Phase 5C part 1 of 2: 360 feedback — the form engine's third consumer
-- (after appraisals' self and manager forms). Two tables:
--   hrm_feedback_cycles   — a feedback campaign, with its own suppression
--                           threshold and form template
--   hrm_feedback_requests — one ask: this respondent, about this subject,
--                           in this relationship
--
-- Design notes:
--
--   • ANONYMITY IS NOT A STORED FLAG. This is the central decision, and it
--     is a deliberate departure from hrm_complaints.is_anonymous — a boolean
--     column carrying a COMMENT that promises "complainant identity hidden
--     from non-HR users at handler level" which NOTHING in the codebase
--     branches on. Grep it: it is selected, inserted and returned, and never
--     once read to make a decision. It is anonymity documented but not
--     implemented, and this migration must not repeat it.
--
--     Instead the policy is DERIVED from the `relationship` column, in Go,
--     by Relationship.IsAnonymous(). A derived policy cannot be set to a
--     value that lies about what the system actually does, and a row cannot
--     be inserted claiming an anonymity it will not receive.
--
--   • self and manager feedback are ATTRIBUTED, BY NATURE — not by
--     oversight. A subject knows what they themselves wrote, and knows who
--     their own manager is. There is exactly one manager, so "anonymous
--     manager feedback" identifies the manager with certainty while
--     pretending otherwise. Suppressing it would add no privacy and would
--     make the most useful feedback in the cycle unreadable. peer,
--     direct_report and external are the anonymous groups.
--
--     Recorded here because the tempting "fix" — anonymising every group
--     uniformly — looks like a hardening and is actually a regression.
--
--   • min_responses lives on the CYCLE, per campaign, defaulting to 3. The
--     hrm_goal_cycles.weight_target precedent: a policy number an org may
--     legitimately differ on is data, not code. Suppression is evaluated
--     PER RELATIONSHIP GROUP, not across the cycle: five responses of which
--     exactly one is a direct report still identify that direct report the
--     moment a "direct reports said" breakdown renders. Aggregating across
--     groups would satisfy a total threshold while leaking the individual.
--
--   • form_instance_id → platform_form_instances ON DELETE SET NULL. The
--     answers live in the form engine; this table owns only WHO WAS ASKED.
--     Splitting it that way is what makes the two read paths (coordination:
--     identity, no content — subject: content, no identity) enforceable as
--     separate queries rather than as a filter someone forgets to apply.
--
--     Note the consequence: a form instance id must NEVER be handed to a
--     subject for someone else's response. platform_form_instances stores
--     respondent_user_id, so an id plus GET /forms/instances/:id defeats the
--     anonymity entirely. The feedback service reads instances server-side
--     and returns answer content, never a reference. Restated in the Go
--     package doc, because it is the one leak that lives outside this module.
--
--   • uq_hrm_fbr_cycle_subject_respondent prevents asking the same person
--     twice about the same subject in one cycle — a duplicate would let one
--     respondent's view count twice toward a suppression threshold designed
--     to require several distinct people.
--
--   • respondent_employee_id is nullable so an EXTERNAL respondent (a
--     customer, a contractor) can be asked without an employee record.
--     respondent_user_id is likewise nullable and independent: an employee
--     may have no platform account. Both nullable, neither derived from the
--     other.
--
-- What must NEVER be added here (the 00076 CHECK × ON DELETE SET NULL trap):
-- a CHECK pairing form_instance_id with status — e.g.
--   CHECK (status <> 'submitted' OR form_instance_id IS NOT NULL)
-- would make DELETE FROM platform_form_instances fail 23514, because
-- Postgres re-evaluates CHECKs on UPDATE and ON DELETE SET NULL is an
-- UPDATE. The submitted_at timestamp carries that fact instead.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_feedback_cycles
-- ------------------------------------------------------------
CREATE TABLE hrm_feedback_cycles (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE DEFAULT 'fbc_' || replace(gen_random_uuid()::text, '-', ''),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name        TEXT        NOT NULL,
    description TEXT,

    period_start DATE       NOT NULL,
    period_end   DATE       NOT NULL,

    -- The questionnaire every request in this cycle instantiates. RESTRICT,
    -- not SET NULL: a cycle whose template vanished cannot instantiate new
    -- requests, and silently becoming un-instantiable mid-campaign is worse
    -- than refusing the delete.
    form_template_id UUID   NOT NULL REFERENCES platform_form_templates(id) ON DELETE RESTRICT,

    -- Responses required in a RELATIONSHIP GROUP before that group's
    -- aggregate renders. Below it the group reports only that it is
    -- suppressed — not its count, which is itself a signal.
    min_responses INTEGER   NOT NULL DEFAULT 3 CHECK (min_responses >= 1),

    status      TEXT        NOT NULL DEFAULT 'draft'
                CHECK (status IN ('draft', 'active', 'closed')),

    closed_at   TIMESTAMPTZ,
    created_by  UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_fbc_period CHECK (period_end >= period_start)
);

CREATE UNIQUE INDEX uq_hrm_fbc_org_name ON hrm_feedback_cycles (org_id, LOWER(name));
CREATE INDEX idx_hrm_fbc_org_status ON hrm_feedback_cycles (org_id, status);

COMMENT ON TABLE hrm_feedback_cycles IS '360 feedback campaign; min_responses is the per-relationship-group suppression threshold, not a cycle-wide total';
COMMENT ON COLUMN hrm_feedback_cycles.min_responses IS 'Responses required WITHIN one relationship group before its aggregate renders; a cycle-wide total would let a single direct report be identified';

-- ------------------------------------------------------------
-- hrm_feedback_requests
--
-- One row = one person asked about one subject. Owns identity and status.
-- Owns no answers: those live in platform_form_instances, reachable only
-- through the service, never by handing the subject an instance id.
-- ------------------------------------------------------------
CREATE TABLE hrm_feedback_requests (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE DEFAULT 'fbr_' || replace(gen_random_uuid()::text, '-', ''),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    cycle_id    UUID        NOT NULL REFERENCES hrm_feedback_cycles(id) ON DELETE CASCADE,

    -- NOT NULL is load-bearing: scope.Predicate emits
    -- subject_employee_id = (SELECT ...), and NULL makes that expression
    -- NULL rather than FALSE, so the row would silently vanish from every
    -- non-ScopeAll list instead of being denied. The hrm_goals.employee_id
    -- precedent.
    subject_employee_id UUID NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    -- Both nullable and independent. An external respondent has no employee
    -- row; an employee respondent may have no platform account.
    respondent_employee_id UUID REFERENCES hrm_employees(id) ON DELETE SET NULL,
    respondent_user_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    -- Snapshotted so a coordination view survives the respondent's employee
    -- row being deleted. The hrm_application_stage_history precedent.
    respondent_name    TEXT NOT NULL,
    respondent_email   TEXT,

    -- Drives the anonymity policy in Go. self and manager are attributed by
    -- nature; the rest are anonymous. See this migration's header.
    relationship TEXT NOT NULL
                 CHECK (relationship IN ('self', 'manager', 'peer', 'direct_report', 'external')),

    form_instance_id UUID REFERENCES platform_form_instances(id) ON DELETE SET NULL,

    status TEXT NOT NULL DEFAULT 'pending'
           CHECK (status IN ('pending', 'submitted', 'declined', 'cancelled')),

    submitted_at   TIMESTAMPTZ,
    declined_at    TIMESTAMPTZ,
    decline_reason TEXT,

    requested_by UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- A self-review is by definition the subject reviewing themselves; any
    -- other pairing under that relationship is a mislabelled peer response
    -- that would then be treated as attributed.
    CONSTRAINT chk_hrm_fbr_self CHECK (
        relationship <> 'self' OR respondent_employee_id = subject_employee_id
    )
);

-- A respondent must not count twice toward a threshold that exists to require
-- several DISTINCT people. Two partial indexes rather than one: internal
-- respondents are keyed by employee id, external ones (who have none) by
-- email, and a single index over a COALESCE of the two would collide every
-- external respondent onto one key.
CREATE UNIQUE INDEX uq_hrm_fbr_cycle_subject_respondent
    ON hrm_feedback_requests (cycle_id, subject_employee_id, respondent_employee_id)
    WHERE respondent_employee_id IS NOT NULL;

CREATE UNIQUE INDEX uq_hrm_fbr_cycle_subject_external
    ON hrm_feedback_requests (cycle_id, subject_employee_id, LOWER(respondent_email))
    WHERE respondent_employee_id IS NULL AND respondent_email IS NOT NULL;

-- The subject-facing read path: every response about one person in one
-- cycle, grouped by relationship. This is the aggregate query's access path.
CREATE INDEX idx_hrm_fbr_subject ON hrm_feedback_requests (cycle_id, subject_employee_id, relationship);
-- The coordination read path: who still owes a response.
CREATE INDEX idx_hrm_fbr_status ON hrm_feedback_requests (cycle_id, status);
-- The respondent's own inbox.
CREATE INDEX idx_hrm_fbr_respondent ON hrm_feedback_requests (respondent_user_id)
    WHERE respondent_user_id IS NOT NULL;

COMMENT ON TABLE hrm_feedback_requests IS 'One 360 ask. Owns WHO was asked; answers live in platform_form_instances and are never reachable by handing a subject an instance id';
COMMENT ON COLUMN hrm_feedback_requests.relationship IS 'Drives anonymity in Go (Relationship.IsAnonymous): self and manager are attributed by nature, peer/direct_report/external are anonymous. Deliberately NOT a stored is_anonymous flag — see hrm_complaints.is_anonymous for what that becomes';
COMMENT ON COLUMN hrm_feedback_requests.form_instance_id IS 'Read server-side only. Handing this id to a subject defeats anonymity, because platform_form_instances stores respondent_user_id';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_feedback_requests;
DROP TABLE IF EXISTS hrm_feedback_cycles;

-- +goose StatementEnd
