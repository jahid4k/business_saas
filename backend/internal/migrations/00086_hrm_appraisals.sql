-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00086_hrm_appraisals
--
-- Phase 5B part 2 of 2: appraisal cycles — the form engine's first real
-- consumer, shipped in the same phase as the engine itself (migration 00084)
-- so the primitive is never speculative. Five tables:
--   hrm_rating_scales        — org-configured rating vocabulary
--   hrm_rating_scale_levels  — its ordered levels, each with a numeric value
--   hrm_appraisal_cycles     — a review round
--   hrm_appraisals           — one employee's appraisal within a cycle
--   hrm_appraisal_phase_history — append-only transition + calibration audit
--
-- Plus one CHECK widening: hrm_acknowledgements.acknowledgeable_type gains
-- 'appraisal', so the acknowledged phase reuses the existing acknowledgement
-- machinery rather than growing a parallel one. The build plan already
-- schedules the identical move for Phase 6's 'course_completion', so widening
-- this CHECK is the sanctioned pattern, not a workaround.
--
-- Design notes:
--
--   • final_rating is a STRUCTURED, QUERYABLE FK — hrm_appraisals
--     .final_rating_level_id → hrm_rating_scale_levels. The build plan
--     (line 276) requires this because Phase 7's merit matrix and Phase 10's
--     9-box both read it; a free-text or bare-integer rating would force a
--     migration in two later phases.
--
--     It is ALSO snapshotted as final_rating_label + final_rating_value. That
--     is not redundancy: the FK gives queryability, the snapshot gives
--     historical fidelity if a level is later renamed or its value
--     re-pointed. Exactly the hrm_application_stage_history shape, which
--     keeps both from_stage_id and from_stage_name for the same reason.
--
--   • manager_employee_id_snapshot is frozen at instantiation — the
--     platform_checklist_instance_items.assignee_user_id precedent, where the
--     manager is resolved once and stored concrete. A reorg mid-cycle must
--     not silently reassign a review that is already underway.
--
--   • self_score / manager_score / goal_attainment are snapshotted AT PUBLISH,
--     not computed on read. Before publish they are read live from the form
--     engine and from Phase 5A's goals. After publish the appraisal is
--     immutable (the payslip pattern), and an immutable record whose numbers
--     are recomputed from mutable sources is not actually immutable.
--
--     goal_attainment is the concrete Phase 5A → 5B tie-in: the appraisee's
--     own weighted goal attainment for the linked goal cycle. 5A's decision
--     that parent_goal_id is alignment-only (no roll-up) is what makes this
--     number stable — otherwise a subordinate back-dating a check-in would
--     change an already-published appraisal's inputs.
--
--   • The phase machine is draft → self_review → manager_review →
--     calibration → published → acknowledged, plus cancelled. Legal
--     transitions live in ONE declarative map in Go (appraisals_model.go),
--     deliberately unlike every other state machine in this codebase, which
--     uses inline guards. With 6 phases plus backward sends, inline guards
--     would be ~15 scattered checks with no single place to read the legal
--     graph. This is a considered deviation, and Phase 5C's PIP machine
--     should follow it rather than the older style.
--
--   • hrm_appraisal_phase_history is append-only and carries the CALIBRATION
--     audit in the same rows: from_rating_* / to_rating_* are populated when
--     a calibration adjusts a rating, so "who changed this rating, from what,
--     to what, and why" is answerable from one table. The build plan requires
--     calibration adjustments to have a mandatory audit trail.
--
--   • Form instances are referenced by FK (hrm → platform, the correct
--     dependency direction — platform never references hrm). ON DELETE SET
--     NULL, because a deleted form instance must not delete an appraisal.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_rating_scales
-- ------------------------------------------------------------
CREATE TABLE hrm_rating_scales (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE
                                DEFAULT ('rscl_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name        TEXT        NOT NULL,
    description TEXT,
    is_default  BOOLEAN     NOT NULL DEFAULT FALSE,
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,

    created_by  UUID        NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX        idx_hrm_rscl_org_id  ON hrm_rating_scales (org_id);
CREATE UNIQUE INDEX uq_hrm_rscl_default  ON hrm_rating_scales (org_id)
    WHERE is_default AND is_active;
CREATE UNIQUE INDEX uq_hrm_rscl_org_name ON hrm_rating_scales (org_id, LOWER(name));

COMMENT ON TABLE hrm_rating_scales IS 'Org-configured rating vocabulary; snapshotted onto each appraisal so a later edit cannot rewrite history';

-- ------------------------------------------------------------
-- hrm_rating_scale_levels
-- ------------------------------------------------------------
CREATE TABLE hrm_rating_scale_levels (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id     TEXT         NOT NULL UNIQUE
                                   DEFAULT ('rlvl_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    scale_id      UUID         NOT NULL REFERENCES hrm_rating_scales(id) ON DELETE CASCADE,

    label         TEXT         NOT NULL,
    description   TEXT,
    -- The numeric anchor Phase 7's merit matrix and Phase 10's 9-box read.
    -- A label alone is unorderable and uncomputable.
    value         NUMERIC(6,2) NOT NULL,
    display_order INTEGER      NOT NULL DEFAULT 0,
    color         TEXT,

    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX        idx_hrm_rlvl_scale ON hrm_rating_scale_levels (scale_id, display_order);
CREATE UNIQUE INDEX uq_hrm_rlvl_label  ON hrm_rating_scale_levels (scale_id, LOWER(label));

COMMENT ON COLUMN hrm_rating_scale_levels.value IS 'Numeric anchor read by Phase 7 merit matrix and Phase 10 9-box; a label alone is unorderable';

-- ------------------------------------------------------------
-- hrm_appraisal_cycles
-- ------------------------------------------------------------
CREATE TABLE hrm_appraisal_cycles (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id                TEXT        NOT NULL UNIQUE
                                             DEFAULT ('acyc_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                   UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name                     TEXT        NOT NULL,
    description              TEXT,
    period_start             DATE        NOT NULL,
    period_end               DATE        NOT NULL,

    -- The Phase 5A seam, exactly as migration 00082's header anticipated.
    -- Nullable because an appraisal cycle with no goal component is
    -- legitimate; SET NULL for the same reason 5A chose it for parent goals —
    -- deleting a goal cycle must not delete review history.
    goal_cycle_id            UUID        REFERENCES hrm_goal_cycles(id) ON DELETE SET NULL,

    -- RESTRICT: a scale in use by a cycle cannot be deleted out from under
    -- the appraisals that reference its levels.
    rating_scale_id          UUID        NOT NULL REFERENCES hrm_rating_scales(id) ON DELETE RESTRICT,

    -- hrm → platform is the correct dependency direction; platform never
    -- references hrm. At least one is required, enforced in the service.
    self_form_template_id    UUID        REFERENCES platform_form_templates(id) ON DELETE SET NULL,
    manager_form_template_id UUID        REFERENCES platform_form_templates(id) ON DELETE SET NULL,

    status                   TEXT        NOT NULL DEFAULT 'draft'
                                             CHECK (status IN ('draft', 'active', 'closed')),

    created_by               UUID        NOT NULL REFERENCES users(id),
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_acyc_period CHECK (period_end >= period_start)
);

CREATE INDEX        idx_hrm_acyc_org_id     ON hrm_appraisal_cycles (org_id);
CREATE INDEX        idx_hrm_acyc_status     ON hrm_appraisal_cycles (org_id, status);
CREATE INDEX        idx_hrm_acyc_goal_cycle ON hrm_appraisal_cycles (goal_cycle_id)
    WHERE goal_cycle_id IS NOT NULL;
CREATE UNIQUE INDEX uq_hrm_acyc_org_name    ON hrm_appraisal_cycles (org_id, LOWER(name));

-- ------------------------------------------------------------
-- hrm_appraisals
-- ------------------------------------------------------------
CREATE TABLE hrm_appraisals (
    id                          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id                   TEXT         NOT NULL UNIQUE
                                                 DEFAULT ('appr_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                      UUID         NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    cycle_id                    UUID         NOT NULL REFERENCES hrm_appraisal_cycles(id) ON DELETE CASCADE,

    -- NOT NULL and load-bearing, exactly as in hrm_goals: internal/hrm/scope's
    -- Predicate filters on this column, and a NULL would make the predicate
    -- NULL rather than FALSE — the row would silently vanish from every
    -- non-ScopeAll list instead of erroring.
    employee_id                 UUID         NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    -- Frozen at instantiation. A reorg mid-cycle must not silently reassign a
    -- review already underway.
    manager_employee_id_snapshot UUID        REFERENCES hrm_employees(id) ON DELETE SET NULL,

    self_form_instance_id       UUID         REFERENCES platform_form_instances(id) ON DELETE SET NULL,
    manager_form_instance_id    UUID         REFERENCES platform_form_instances(id) ON DELETE SET NULL,

    phase                       TEXT         NOT NULL DEFAULT 'draft'
                                                 CHECK (phase IN ('draft', 'self_review', 'manager_review',
                                                                  'calibration', 'published', 'acknowledged',
                                                                  'cancelled')),

    -- Structured and queryable for Phase 7 / Phase 10 ...
    final_rating_level_id       UUID         REFERENCES hrm_rating_scale_levels(id) ON DELETE SET NULL,
    -- ... and snapshotted, so a renamed or re-valued level cannot rewrite a
    -- published appraisal.
    final_rating_label          TEXT,
    final_rating_value          NUMERIC(6,2),

    -- Snapshotted at publish; read live from their sources before then.
    self_score                  NUMERIC(6,2),
    manager_score               NUMERIC(6,2),
    goal_attainment             NUMERIC(6,2),

    published_at                TIMESTAMPTZ,
    published_by                UUID         REFERENCES users(id) ON DELETE SET NULL,
    acknowledged_at             TIMESTAMPTZ,
    cancelled_at                TIMESTAMPTZ,
    cancel_reason               TEXT,

    created_by                  UUID         NOT NULL REFERENCES users(id),
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_appr_org_id   ON hrm_appraisals (org_id);
CREATE INDEX idx_hrm_appr_employee ON hrm_appraisals (employee_id);
CREATE INDEX idx_hrm_appr_cycle    ON hrm_appraisals (cycle_id);
CREATE INDEX idx_hrm_appr_phase    ON hrm_appraisals (org_id, phase);
CREATE INDEX idx_hrm_appr_manager  ON hrm_appraisals (manager_employee_id_snapshot)
    WHERE manager_employee_id_snapshot IS NOT NULL;
-- Phase 7's merit matrix and Phase 10's 9-box both read final_rating_level_id.
CREATE INDEX idx_hrm_appr_rating   ON hrm_appraisals (final_rating_level_id)
    WHERE final_rating_level_id IS NOT NULL;
-- One appraisal per employee per cycle.
CREATE UNIQUE INDEX uq_hrm_appr_cycle_employee ON hrm_appraisals (cycle_id, employee_id);

COMMENT ON COLUMN hrm_appraisals.final_rating_level_id IS 'Structured FK for Phase 7 merit matrix / Phase 10 9-box; paired with the label+value snapshot for historical fidelity';
COMMENT ON COLUMN hrm_appraisals.manager_employee_id_snapshot IS 'Frozen at instantiation so a mid-cycle reorg cannot reassign a review already underway';
COMMENT ON COLUMN hrm_appraisals.goal_attainment IS 'Appraisee OWN weighted goal attainment from the linked goal cycle, snapshotted at publish';

-- ------------------------------------------------------------
-- hrm_appraisal_phase_history
-- ------------------------------------------------------------
CREATE TABLE hrm_appraisal_phase_history (
    id                    UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id             TEXT         NOT NULL UNIQUE
                                           DEFAULT ('aphs_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    -- No org_id: reached via appraisal_id, the hrm_application_stage_history
    -- precedent.
    appraisal_id          UUID         NOT NULL REFERENCES hrm_appraisals(id) ON DELETE CASCADE,

    from_phase            TEXT,
    to_phase              TEXT         NOT NULL,

    -- The calibration audit lives in these columns rather than a second
    -- table: a calibration IS a transition, and splitting them would make
    -- "who changed this rating and why" a two-table question.
    -- Snapshotted labels so a renamed level cannot rewrite the record.
    from_rating_level_id  UUID         REFERENCES hrm_rating_scale_levels(id) ON DELETE SET NULL,
    from_rating_label     TEXT,
    to_rating_level_id    UUID         REFERENCES hrm_rating_scale_levels(id) ON DELETE SET NULL,
    to_rating_label       TEXT,

    note                  TEXT,
    changed_by            UUID         REFERENCES users(id) ON DELETE SET NULL,
    changed_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW()
    -- Append-only: deliberately no updated_at.
);

CREATE INDEX idx_hrm_aphs_appraisal ON hrm_appraisal_phase_history (appraisal_id, changed_at DESC);

COMMENT ON TABLE hrm_appraisal_phase_history IS 'Append-only phase transitions AND the mandatory calibration audit; a calibration is a transition, so both live in one table';

-- ------------------------------------------------------------
-- hrm_acknowledgements: register 'appraisal'
--
-- The acknowledged phase reuses the existing acknowledgement machinery
-- rather than growing a parallel one. The build plan already schedules the
-- identical widening for Phase 6's 'course_completion'.
-- ------------------------------------------------------------
ALTER TABLE hrm_acknowledgements
    DROP CONSTRAINT IF EXISTS hrm_acknowledgements_acknowledgeable_type_check,
    ADD CONSTRAINT hrm_acknowledgements_acknowledgeable_type_check
        CHECK (acknowledgeable_type IN (
            'warning', 'document', 'announcement', 'calendar_event', 'policy', 'appraisal'
        ));

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE hrm_acknowledgements
    DROP CONSTRAINT IF EXISTS hrm_acknowledgements_acknowledgeable_type_check,
    ADD CONSTRAINT hrm_acknowledgements_acknowledgeable_type_check
        CHECK (acknowledgeable_type IN (
            'warning', 'document', 'announcement', 'calendar_event', 'policy'
        ));

DROP TABLE IF EXISTS hrm_appraisal_phase_history;
DROP TABLE IF EXISTS hrm_appraisals;
DROP TABLE IF EXISTS hrm_appraisal_cycles;
DROP TABLE IF EXISTS hrm_rating_scale_levels;
DROP TABLE IF EXISTS hrm_rating_scales;

-- +goose StatementEnd
