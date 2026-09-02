-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00082_hrm_goals
--
-- Phase 5A of the HRM Extended Build Plan: Goals / OKR. Three tables:
--   hrm_goal_cycles    — the org-scoped period goals belong to
--   hrm_goals          — the goal itself
--   hrm_goal_checkins  — append-only progress history
--
-- Phase 5 as written in the build plan bundles a form engine plus four
-- Performance Management sub-systems (~18-19 tables). It is sliced: 5A is
-- Goals/OKR, which is the ONLY sub-system with no form-engine dependency, so
-- it ships without forcing a speculative primitive. The form engine lands in
-- 5B alongside appraisals, its first real consumer — the same shape as Phase
-- 3, where 00076's checklist engine shipped with onboarding rather than ahead
-- of it.
--
-- Design notes:
--
--   • parent_goal_id is ON DELETE SET NULL, not CASCADE. The build plan's
--     phrase is "parent_goal_id self-FK cascade", but "cascade" there is the
--     OKR domain term (cascading goals = top-down alignment), not DDL: the
--     same sentence demands "hrm_goal_checkins history from day one", and
--     ON DELETE CASCADE is the single choice that destroys the most of that
--     history — one deleted company objective would take every aligned goal
--     in the org and, through this table's own cascade, every check-in under
--     them. The codebase's only self-FK precedent agrees:
--     hrm_departments.parent_department_id (00021) is ON DELETE SET NULL.
--     The failure modes are asymmetric — SET NULL orphans a few goals
--     cosmetically; CASCADE silently destroys a tree with no undo.
--
--   • parent_goal_id expresses ALIGNMENT ONLY. A parent's current_value is
--     never derived from its children. Roll-up would be a denormalized
--     aggregate that drifts (00076 forbids exactly that), would need a
--     recursive WRITE path on a tree with no DB-level acyclicity guard, and
--     would break 5B: appraisals are publish-immutable (the payslip pattern),
--     so a subordinate back-dating a check-in must not mutate the inputs of
--     an already-published appraisal. Aggregate progress across children is
--     computed on read, never stored.
--
--   • Objective vs key result is NOT discriminated by parent_goal_id IS NULL.
--     Real OKR trees run company → department → individual → key results, so
--     that test misclassifies a department objective the moment the feature is
--     used as intended. goal_level carries hierarchy; weight IS NULL carries
--     "tracking only, not appraised". Encoding the appraised/not distinction
--     as the nullability of the very column the weight guard reads means the
--     two cannot drift apart.
--
--   • employee_id is NOT NULL, and that is load-bearing rather than
--     incidental. internal/hrm/scope's Predicate emits
--     `employee_id = (SELECT id FROM hrm_employees WHERE ...)`; a NULL column
--     makes that expression NULL rather than FALSE, so the row would silently
--     vanish from every non-ScopeAll list instead of erroring. A company
--     objective therefore has a sponsoring employee — it is not an ownerless
--     row.
--
--   • Progress percent is NOT stored on hrm_goals. It is a pure function of
--     start_value / current_value / target_value / measurement_type — the
--     00076 rule ("completion percentage is ALWAYS computed ... no
--     denormalized counter to drift"). It IS stored on hrm_goal_checkins,
--     which is not a contradiction: that is a historical value that must
--     never change again, the same distinction that lets hrm_leave_balances
--     (00074) store immutable snapshots while current balance stays computed.
--
--   • current_value is mutable ONLY through a check-in. UpdateGoal ignores the
--     column outright, so hrm_goal_checkins can never have holes.
--
-- ⚠ CHECK × ON DELETE SET NULL — the 00076 trap, and where it does NOT bite:
--   Postgres re-evaluates CHECK constraints on UPDATE, and ON DELETE SET NULL
--   IS an UPDATE. A CHECK pairing two columns therefore breaks DELETE on the
--   referenced table. Both CHECKs below are safe because they degrade to TRUE
--   when the FK column is nulled (chk_hrm_goal_no_self_parent becomes
--   NULL IS NULL). These must NEVER be added:
--       CHECK (status <> 'locked' OR locked_by IS NOT NULL)
--         → DELETE FROM users fails 23514 for any org with a locked cycle.
--       CHECK (goal_level = 'company' OR parent_goal_id IS NOT NULL)
--         → DELETE of any parent goal fails on every child.
--   Both rules belong in the service on write instead; a deleted user degrades
--   locked_by to unknown, the fail-soft path 00076's template items take.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_goal_cycles
-- ------------------------------------------------------------
CREATE TABLE hrm_goal_cycles (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id      TEXT         NOT NULL UNIQUE
                                    DEFAULT ('gcyc_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id         UUID         NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name           TEXT         NOT NULL,
    description    TEXT,
    period_start   DATE         NOT NULL,
    period_end     DATE         NOT NULL,

    -- Lifecycle mirrors hrm_attendance_periods (00040), which payslips already
    -- uses as an upstream gate. 'locked' freezes goal DEFINITIONS while
    -- check-ins keep landing — the normal in-flight state for a quarter.
    -- 'closed' is fully immutable.
    status         TEXT         NOT NULL DEFAULT 'draft'
                                    CHECK (status IN ('draft', 'active', 'locked', 'closed')),

    -- The weight-sum denominator, per cycle rather than a Go constant, so an
    -- org running a 10-point weighting scheme is a data change not a code one.
    weight_target  NUMERIC(6,2) NOT NULL DEFAULT 100 CHECK (weight_target > 0),

    locked_at      TIMESTAMPTZ,
    locked_by      UUID         REFERENCES users(id) ON DELETE SET NULL,
    closed_at      TIMESTAMPTZ,

    created_by     UUID         NOT NULL REFERENCES users(id),
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_gcyc_period CHECK (period_end >= period_start)
);

CREATE INDEX        idx_hrm_gcyc_org_id  ON hrm_goal_cycles (org_id);
CREATE INDEX        idx_hrm_gcyc_status  ON hrm_goal_cycles (org_id, status);
CREATE INDEX        idx_hrm_gcyc_period  ON hrm_goal_cycles (org_id, period_start DESC);
CREATE UNIQUE INDEX uq_hrm_gcyc_org_name ON hrm_goal_cycles (org_id, LOWER(name));

COMMENT ON TABLE  hrm_goal_cycles IS 'Org-scoped goal period; the scope key for weight-sum validation';
COMMENT ON COLUMN hrm_goal_cycles.weight_target IS 'Weight denominator for this cycle; goals may total at most this, and must total exactly this to lock';
COMMENT ON COLUMN hrm_goal_cycles.status IS 'draft = configuring; active = goals writable; locked = definitions frozen, check-ins still allowed; closed = immutable';

-- Deliberately NO constraint forbidding overlapping periods: an annual and a
-- quarterly cycle running concurrently is normal OKR practice, and 5B's
-- appraisal cycle will want to span two goal cycles.

-- ------------------------------------------------------------
-- hrm_goals
-- ------------------------------------------------------------
CREATE TABLE hrm_goals (
    id                UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id         TEXT          NOT NULL UNIQUE
                                        DEFAULT ('goal_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id            UUID          NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    -- RESTRICT so a cycle with attached goals cannot be deleted out from under
    -- them, which would otherwise route around the goal-level history guard.
    cycle_id          UUID          NOT NULL REFERENCES hrm_goal_cycles(id) ON DELETE RESTRICT,
    employee_id       UUID          NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,
    parent_goal_id    UUID          REFERENCES hrm_goals(id) ON DELETE SET NULL,

    title             TEXT          NOT NULL,
    description       TEXT,
    goal_level        TEXT          NOT NULL DEFAULT 'individual'
                                        CHECK (goal_level IN ('individual', 'team', 'department', 'company')),
    -- Free text in 5A. A managed taxonomy is Phase 6's hrm_skills, which the
    -- build plan designates a SHARED taxonomy consumed by Phases 4, 5 and 10 —
    -- "not an LMS-internal table". Building a goals-local one now is exactly
    -- the speculative primitive this slicing exists to avoid; Phase 6 adds
    -- competency_id as a pure ALTER.
    category          TEXT,

    measurement_type  TEXT          NOT NULL DEFAULT 'percentage'
                                        CHECK (measurement_type IN ('percentage', 'numeric', 'currency', 'boolean')),
    -- Validation and display only. It is NOT an input to the progress
    -- formula: start_value already makes that arithmetic direction-agnostic,
    -- and branching on direction is how the obvious implementation gets the
    -- sign wrong on a decrease goal.
    direction         TEXT          NOT NULL DEFAULT 'increase'
                                        CHECK (direction IN ('increase', 'decrease')),
    start_value       NUMERIC(18,4) NOT NULL DEFAULT 0,
    target_value      NUMERIC(18,4) NOT NULL DEFAULT 100,
    current_value     NUMERIC(18,4) NOT NULL DEFAULT 0,
    unit              TEXT,
    currency_code     CHAR(3),

    weight            NUMERIC(6,2)  CHECK (weight IS NULL OR (weight >= 0 AND weight <= 100)),

    status            TEXT          NOT NULL DEFAULT 'draft'
                                        CHECK (status IN ('draft', 'active', 'completed', 'cancelled')),
    -- Nullable outcome alongside status, the hrm_interviews (00080) shape.
    -- Recorded when a goal is completed; never auto-derived from progress.
    outcome           TEXT          CHECK (outcome IN ('exceeded', 'achieved', 'partially_achieved', 'missed')),

    start_date        DATE,
    due_date          DATE,
    completed_at      TIMESTAMPTZ,
    cancelled_at      TIMESTAMPTZ,
    cancel_reason     TEXT,

    created_by        UUID          NOT NULL REFERENCES users(id),
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_goal_no_self_parent CHECK (parent_goal_id IS NULL OR parent_goal_id <> id),
    CONSTRAINT chk_hrm_goal_dates          CHECK (due_date IS NULL OR start_date IS NULL OR due_date >= start_date)
);

CREATE INDEX idx_hrm_goal_org_id    ON hrm_goals (org_id);
CREATE INDEX idx_hrm_goal_employee  ON hrm_goals (employee_id);
CREATE INDEX idx_hrm_goal_cycle     ON hrm_goals (cycle_id);
CREATE INDEX idx_hrm_goal_parent    ON hrm_goals (parent_goal_id) WHERE parent_goal_id IS NOT NULL;
CREATE INDEX idx_hrm_goal_status    ON hrm_goals (org_id, status);
-- The weight guard's exact access path, and 5B's "goals for this employee in
-- this cycle" appraisal join.
CREATE INDEX idx_hrm_goal_emp_cycle ON hrm_goals (employee_id, cycle_id);

COMMENT ON TABLE  hrm_goals IS 'Goals / OKRs. One table for objectives and key results, discriminated by goal_level and weight IS NULL — never by parent_goal_id IS NULL';
COMMENT ON COLUMN hrm_goals.parent_goal_id IS 'Alignment only: this goal supports that one. Progress NEVER rolls up into the parent';
COMMENT ON COLUMN hrm_goals.employee_id IS 'NOT NULL by design — scope.Predicate filters on this column, and NULL would evaluate to NULL (row vanishes) rather than FALSE';
COMMENT ON COLUMN hrm_goals.weight IS 'NULL = tracking only, excluded from the cycle weight total; lets an objective and its key results coexist without double-counting';
COMMENT ON COLUMN hrm_goals.current_value IS 'Mutable ONLY through a check-in, so hrm_goal_checkins can never have holes';
COMMENT ON COLUMN hrm_goals.direction IS 'Validation and display only; the progress formula is direction-agnostic via start_value';

-- ------------------------------------------------------------
-- hrm_goal_checkins
-- ------------------------------------------------------------
CREATE TABLE hrm_goal_checkins (
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id        TEXT          NOT NULL UNIQUE
                                       DEFAULT ('gchk_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    -- No org_id: reached via goal_id → hrm_goals, the
    -- hrm_application_stage_history (00078) / hrm_interview_scorecards (00080)
    -- precedent.
    goal_id          UUID          NOT NULL REFERENCES hrm_goals(id) ON DELETE CASCADE,

    previous_value   NUMERIC(18,4) NOT NULL,
    current_value    NUMERIC(18,4) NOT NULL,
    -- Derived at write time inside the same transaction as the value change,
    -- exactly like hrm_application_stage_history.seconds_in_previous_stage.
    -- Stored UNCLAMPED: overshoot beyond 100 and regression below 0 are both
    -- real facts, and clamping is a read-side concern history must not lose.
    progress_percent NUMERIC(9,2)  NOT NULL,

    -- Snapshotted so a later status rename cannot rewrite what was reported at
    -- the time — the from_stage_name precedent.
    status_snapshot  TEXT          NOT NULL,
    confidence       TEXT          CHECK (confidence IN ('on_track', 'at_risk', 'off_track')),
    note             TEXT,

    checked_in_by    UUID          REFERENCES users(id) ON DELETE SET NULL,
    checked_in_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
    -- Append-only: deliberately no updated_at. Nothing ever UPDATEs this table.
);

CREATE INDEX idx_hrm_gchk_goal_id ON hrm_goal_checkins (goal_id, checked_in_at DESC);

COMMENT ON TABLE  hrm_goal_checkins IS 'Append-only goal progress history; the only path that mutates hrm_goals.current_value';
COMMENT ON COLUMN hrm_goal_checkins.progress_percent IS 'Unclamped snapshot derived at write time; immutable history, which is why storing it does not violate the computed-not-stored rule';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_goal_checkins;
DROP TABLE IF EXISTS hrm_goals;
DROP TABLE IF EXISTS hrm_goal_cycles;

-- +goose StatementEnd
