-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00090_hrm_pips
--
-- Phase 5C part 2 of 2: performance improvement plans. Two tables:
--   hrm_pips           — the plan: what must improve, by when, outcome
--   hrm_pip_checkins   — append-only review history over the plan's life
--
-- Design notes:
--
--   • A FAILED PIP CREATES A DRAFT TERMINATION AND STOPS. It does not
--     terminate anyone. termination_id is nullable and set by that handoff,
--     the recruitment → employees hire seam applied in reverse: the PIP
--     service declares a narrow TerminationCreator interface and
--     terminations.Service satisfies it structurally.
--
--     Stopping at draft is not timidity, it is this codebase's "no implicit
--     state machine" rule. hrm_terminations already has its own
--     draft → pending_approval → approved → applied lifecycle with an
--     approval chain behind it; a PIP that skipped to `applied` would route
--     around the approval that exists specifically to gate dismissals.
--
--     ON DELETE SET NULL, not RESTRICT: deleting a mistakenly-created draft
--     termination must not be blocked by the PIP that suggested it. The PIP
--     retains outcome='failed' either way, so the record of what happened
--     does not depend on the draft surviving.
--
--   • outcome is SEPARATE from status, and both are needed. status is where
--     the plan is in its lifecycle (draft/active/extended/closed); outcome is
--     how it ended (successful/failed/abandoned), and is NULL until it ends.
--     Fusing them yields one CHECK with seven values of which several are
--     illegal in combination with the dates — the same reasoning that kept
--     hrm_goal_cycles separate from hrm_appraisal_cycles in 00082.
--
--   • end_date is extendable, and every extension is a hrm_pip_checkins row.
--     original_end_date is frozen at creation so "this PIP was extended
--     twice" stays legible after the fact. A PIP whose end date silently
--     moves is the documented failure mode of the whole instrument.
--
--   • hrm_pip_checkins has no org_id — it is reached through pip_id, the
--     hrm_goal_checkins / hrm_application_stage_history precedent. It has no
--     updated_at either, because nothing ever UPDATEs it: a review note that
--     can be edited after a dismissal is not evidence.
--
--   • manager_employee_id is who owns the plan, snapshotted at creation for
--     the reason appraisals snapshot theirs — a reorg mid-plan must not
--     silently reassign responsibility for someone's dismissal process.
--
-- What must NEVER be added here (the 00076 CHECK × ON DELETE SET NULL trap):
-- a CHECK pairing outcome with termination_id — e.g.
--   CHECK (outcome <> 'failed' OR termination_id IS NOT NULL)
-- would make DELETE FROM hrm_terminations fail 23514 on any org with a
-- failed PIP, because Postgres re-evaluates CHECKs on UPDATE and ON DELETE
-- SET NULL is an UPDATE. Every CHECK below reads one column or a pair that
-- cannot be nulled by a delete.
-- ============================================================

-- ------------------------------------------------------------
-- hrm_pips
-- ------------------------------------------------------------
CREATE TABLE hrm_pips (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE DEFAULT 'pip_' || replace(gen_random_uuid()::text, '-', ''),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- NOT NULL is load-bearing: scope.Predicate emits
    -- employee_id = (SELECT ...), and NULL makes that expression NULL rather
    -- than FALSE, so the row would vanish from every non-ScopeAll list
    -- instead of being denied. The hrm_goals.employee_id precedent.
    employee_id UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    -- Frozen at creation. A reorg must not silently hand someone else's
    -- dismissal process to a manager who never opened it.
    manager_employee_id UUID REFERENCES hrm_employees(id) ON DELETE SET NULL,

    title       TEXT        NOT NULL,
    -- The concerns being raised and what success looks like. Both required:
    -- a PIP without stated success criteria is unmeetable by construction.
    concerns          TEXT  NOT NULL,
    success_criteria  TEXT  NOT NULL,
    support_provided  TEXT,

    start_date         DATE NOT NULL,
    end_date           DATE NOT NULL,
    -- Frozen at creation so extensions stay visible after the fact.
    original_end_date  DATE NOT NULL,

    status TEXT NOT NULL DEFAULT 'draft'
           CHECK (status IN ('draft', 'active', 'extended', 'closed', 'cancelled')),

    -- NULL until the plan ends. Separate from status by design — see header.
    outcome TEXT CHECK (outcome IN ('successful', 'failed', 'abandoned')),

    -- Set by the failed-PIP handoff. SET NULL so deleting a mistaken draft
    -- termination is not blocked by the PIP that suggested it.
    termination_id UUID REFERENCES hrm_terminations(id) ON DELETE SET NULL,

    closed_at   TIMESTAMPTZ,
    closed_by   UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_by  UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hrm_pip_dates     CHECK (end_date >= start_date),
    CONSTRAINT chk_hrm_pip_extension CHECK (end_date >= original_end_date)
);

CREATE INDEX idx_hrm_pip_employee ON hrm_pips (employee_id, start_date DESC);
CREATE INDEX idx_hrm_pip_org_status ON hrm_pips (org_id, status);
CREATE INDEX idx_hrm_pip_manager ON hrm_pips (manager_employee_id)
    WHERE manager_employee_id IS NOT NULL;

-- One open plan per employee. A second concurrent PIP is always a data-entry
-- error, and two overlapping plans make "did they pass" unanswerable.
CREATE UNIQUE INDEX uq_hrm_pip_employee_open ON hrm_pips (employee_id)
    WHERE status IN ('draft', 'active', 'extended');

COMMENT ON TABLE hrm_pips IS 'Performance improvement plan; a failed outcome creates a DRAFT hrm_terminations row and stops, never an applied termination';
COMMENT ON COLUMN hrm_pips.original_end_date IS 'Frozen at creation so extensions remain visible; end_date moves, this does not';
COMMENT ON COLUMN hrm_pips.termination_id IS 'Draft termination created by the failed-PIP handoff; SET NULL so deleting a mistaken draft is not blocked';

-- ------------------------------------------------------------
-- hrm_pip_checkins
--
-- Append-only. No org_id (reached via pip_id), no updated_at (nothing ever
-- UPDATEs it). A review note editable after a dismissal is not evidence.
-- ------------------------------------------------------------
CREATE TABLE hrm_pip_checkins (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE DEFAULT 'pipc_' || replace(gen_random_uuid()::text, '-', ''),
    pip_id      UUID        NOT NULL REFERENCES hrm_pips(id) ON DELETE CASCADE,

    -- 'review' is a scheduled progress check; 'extension' records the end
    -- date moving and by how much; 'closure' is the final entry.
    entry_type  TEXT        NOT NULL DEFAULT 'review'
                CHECK (entry_type IN ('review', 'extension', 'closure')),

    -- The manager's read at this point in the plan. Advisory, not the
    -- outcome — the outcome is decided once, on hrm_pips.
    progress    TEXT        CHECK (progress IN ('on_track', 'partial', 'off_track')),
    note        TEXT        NOT NULL,

    -- Populated on entry_type='extension' so the extension history is
    -- readable without diffing rows.
    previous_end_date DATE,
    new_end_date      DATE,

    checked_in_by UUID        REFERENCES users(id) ON DELETE SET NULL,
    checked_in_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_pipc_pip ON hrm_pip_checkins (pip_id, checked_in_at DESC);

COMMENT ON TABLE hrm_pip_checkins IS 'Append-only PIP review history; no updated_at because a note editable after a dismissal is not evidence';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_pip_checkins;
DROP TABLE IF EXISTS hrm_pips;

-- +goose StatementEnd
