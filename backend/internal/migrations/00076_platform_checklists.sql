-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00076_platform_checklists
--
-- Phase 3 of the HRM Extended Build Plan: a shared platform checklist
-- engine (platform_checklist_*) whose first consumer is HRM Onboarding.
-- Phase 9 (Exit Management) will later reuse checklist_type='offboarding'.
--
-- Four tables:
--   platform_checklist_templates       — org-defined checklist configs
--   platform_checklist_template_items  — items within a template
--   platform_checklist_instances       — a template applied to a subject
--   platform_checklist_instance_items  — per-item runtime state (snapshot)
--
-- Design notes:
--   • org_id is NOT NULL on templates — unlike `roles`, a global
--     (org_id IS NULL) template cannot express owner_type='specific_user'
--     or an org-created owner_type='role', and would force
--     `WHERE (org_id = $1 OR org_id IS NULL)` onto every read. Starter
--     templates, if ever wanted, are a preset-copy endpoint, not a
--     nullable org_id.
--   • owner_type mirrors hrm_approval_template_levels.approver_type
--     ('subject'|'manager'|'role'|'specific_user') rather than literal
--     hr/it/finance roles, which do not exist as seeded system roles.
--   • Instances are polymorphic (subject_type/subject_id, no FK) — the
--     same pattern as hrm_acknowledgements.acknowledgeable_type/_id — so
--     this package never has to reference hrm_employees and stays a true
--     platform primitive. subject_type is deliberately narrow (only
--     'employee' today); Phase 9 widens it, and forgetting to widen it
--     fails loudly via the CHECK rather than silently accepting garbage.
--   • Instance items are column-level snapshots of their template item,
--     not a JSONB blob — unlike hrm_approval_instances.instance_snapshot,
--     items here are individually completed/skipped rows that must be
--     aggregated with COUNT(*) FILTER, so they need to be real columns.
--   • Completion percentage is ALWAYS computed from instance items, never
--     stored — no denormalized counter to drift.
--   • is_blocking is stored and reported but enforced by NOTHING in this
--     phase (onboarding has nothing to gate). Phase 9 adds
--     blocking_amount/_currency as pure ALTERs.
--
-- ⚠ Deliberate asymmetry, read before "fixing": template items have a
--   CHECK pairing owner_role with owner_type='role', but NO CHECK pairing
--   owner_user_id with owner_type='specific_user'. Adding one would make
--   `DELETE FROM users` fail whenever that user owns a template item —
--   the owner_user_id FK's ON DELETE SET NULL is an UPDATE under the
--   hood, and Postgres re-evaluates CHECK constraints on UPDATE, not just
--   INSERT. The service enforces "specific_user needs owner_user_id" on
--   write instead; a deleted user degrades the item to unassigned, the
--   same fail-soft path a role-with-no-holders takes. The same trap
--   applies to instance_items.assignee_user_id/owner_user_id and
--   .completed_by — none of those get a paired CHECK either.
-- ============================================================

-- ------------------------------------------------------------
-- platform_checklist_templates
-- ------------------------------------------------------------
CREATE TABLE platform_checklist_templates (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id      TEXT        NOT NULL UNIQUE
                                   DEFAULT ('clt_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id         UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name           TEXT        NOT NULL,
    description    TEXT,
    checklist_type TEXT        NOT NULL
                                   CHECK (checklist_type IN (
                                       'onboarding', 'offboarding',
                                       'probation_confirmation', 'transfer_handover'
                                   )),

    is_default     BOOLEAN     NOT NULL DEFAULT FALSE,
    is_active      BOOLEAN     NOT NULL DEFAULT TRUE,

    created_by     UUID        NOT NULL REFERENCES users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pchk_tpl_org_id ON platform_checklist_templates (org_id);
CREATE INDEX idx_pchk_tpl_type   ON platform_checklist_templates (org_id, checklist_type);

-- Only one default per org+checklist_type. Mirrors idx_hrm_apt_default —
-- but unlike hrm/approvals' repository (which never clears a prior
-- default and lets a second one raise a raw 23505), this package's
-- service calls ClearDefault() first, inside the same transaction that
-- sets the new one.
CREATE UNIQUE INDEX uq_pchk_tpl_default
    ON platform_checklist_templates (org_id, checklist_type)
    WHERE is_default = TRUE AND is_active = TRUE;

COMMENT ON TABLE  platform_checklist_templates IS 'Org-defined checklist configurations (onboarding, offboarding, etc.)';
COMMENT ON COLUMN platform_checklist_templates.is_default IS 'Exactly one default per org+checklist_type; used by auto-instantiation hooks';

-- ------------------------------------------------------------
-- platform_checklist_template_items
-- ------------------------------------------------------------
CREATE TABLE platform_checklist_template_items (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id           TEXT        NOT NULL UNIQUE
                                        DEFAULT ('cltm_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    template_id         UUID        NOT NULL REFERENCES platform_checklist_templates(id) ON DELETE CASCADE,

    title               TEXT        NOT NULL,
    description         TEXT,

    owner_type          TEXT        NOT NULL
                                        CHECK (owner_type IN ('subject', 'manager', 'role', 'specific_user')),
    owner_role          TEXT,       -- role NAME, only when owner_type = 'role'
    owner_user_id       UUID        REFERENCES users(id) ON DELETE SET NULL,  -- only when owner_type = 'specific_user'

    -- Calendar days from the instance's anchor_date. NEGATIVE IS VALID —
    -- e.g. -3 for a pre-boarding "send welcome email" item. Not clamped.
    due_offset_days     INTEGER     NOT NULL DEFAULT 0,

    -- Reported in progress payloads but enforced by nothing this phase —
    -- onboarding has nothing to gate on it. Honest, not aspirational.
    is_blocking         BOOLEAN     NOT NULL DEFAULT FALSE,
    requires_attachment BOOLEAN     NOT NULL DEFAULT FALSE,

    -- Items are parallel, not sequential like approval levels — no unique
    -- constraint on (template_id, display_order); a unique index would
    -- 23505 on any drag-reorder.
    display_order       INTEGER     NOT NULL DEFAULT 0,
    is_active           BOOLEAN     NOT NULL DEFAULT TRUE,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_pchk_tpl_item_role CHECK (owner_type <> 'role' OR owner_role IS NOT NULL)
);

CREATE INDEX idx_pchk_tpl_item_template_id ON platform_checklist_template_items (template_id);

COMMENT ON TABLE  platform_checklist_template_items IS 'Items within a checklist template';
COMMENT ON COLUMN platform_checklist_template_items.owner_user_id IS 'Deliberately no paired CHECK with owner_type=specific_user — see migration header (SET NULL/CHECK re-evaluation trap). Enforced in the service on write.';
COMMENT ON COLUMN platform_checklist_template_items.is_blocking   IS 'Reported only; not enforced anywhere in Phase 3. Phase 9 (offboarding) adds real enforcement plus blocking_amount.';

-- ------------------------------------------------------------
-- platform_checklist_instances
-- ------------------------------------------------------------
CREATE TABLE platform_checklist_instances (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT        NOT NULL UNIQUE
                                    DEFAULT ('cli_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id          UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    template_id     UUID        REFERENCES platform_checklist_templates(id) ON DELETE SET NULL,

    -- Snapshot of the template at instantiation time — deleting or
    -- renaming the template must not affect a live instance's display.
    template_name   TEXT        NOT NULL,
    checklist_type  TEXT        NOT NULL,

    -- Polymorphic subject — no FK, deliberately (hrm_acknowledgements
    -- precedent). This is what keeps this package free of any hrm_*
    -- dependency. Widen the CHECK, not the shape, when Phase 9 arrives.
    subject_type    TEXT        NOT NULL CHECK (subject_type IN ('employee')),
    subject_id      UUID        NOT NULL,
    subject_label   TEXT        NOT NULL,   -- caller-asserted display name; never dereferenced

    -- Resolved once at instantiation time from the caller's SubjectContext.
    -- NULL means the subject has no platform account (e.g. a contractor).
    subject_user_id UUID        REFERENCES users(id) ON DELETE SET NULL,

    anchor_date     DATE        NOT NULL,   -- hire_date for onboarding

    status          TEXT        NOT NULL DEFAULT 'in_progress'
                                    CHECK (status IN ('in_progress', 'completed', 'cancelled')),
    completed_at    TIMESTAMPTZ,
    cancelled_at    TIMESTAMPTZ,
    cancel_reason   TEXT,

    created_by      UUID        NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pchk_inst_org_id  ON platform_checklist_instances (org_id);
CREATE INDEX idx_pchk_inst_subject ON platform_checklist_instances (org_id, subject_type, subject_id);
CREATE INDEX idx_pchk_inst_status  ON platform_checklist_instances (org_id, status);

COMMENT ON TABLE  platform_checklist_instances IS 'A checklist template applied to a specific subject (e.g. an onboarding employee)';
COMMENT ON COLUMN platform_checklist_instances.subject_id IS 'Opaque UUID to this package — no FK. Resolved and owned entirely by the consuming module (e.g. hrm/onboarding).';

-- ------------------------------------------------------------
-- platform_checklist_instance_items
-- ------------------------------------------------------------
CREATE TABLE platform_checklist_instance_items (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id           TEXT        NOT NULL UNIQUE
                                        DEFAULT ('clim_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    instance_id         UUID        NOT NULL REFERENCES platform_checklist_instances(id) ON DELETE CASCADE,
    template_item_id    UUID        REFERENCES platform_checklist_template_items(id) ON DELETE SET NULL,  -- provenance only, never joined for display

    -- Snapshot of the template item at instantiation time.
    title               TEXT        NOT NULL,
    description         TEXT,
    owner_type          TEXT        NOT NULL
                                        CHECK (owner_type IN ('subject', 'manager', 'role', 'specific_user')),
    owner_role          TEXT,
    is_blocking         BOOLEAN     NOT NULL DEFAULT FALSE,
    requires_attachment BOOLEAN     NOT NULL DEFAULT FALSE,
    display_order       INTEGER     NOT NULL DEFAULT 0,
    due_offset_days     INTEGER     NOT NULL DEFAULT 0,

    -- Resolved once at instantiation time. NULL = role-owned group claim
    -- (owner_type='role') or an unresolvable owner (deleted user, absent
    -- manager) — see hrm/onboarding's resolveAssignee for the matrix.
    assignee_user_id    UUID        REFERENCES users(id) ON DELETE SET NULL,

    due_date            DATE,       -- anchor_date + due_offset_days, frozen at instantiation

    status              TEXT        NOT NULL DEFAULT 'pending'
                                        CHECK (status IN ('pending', 'completed', 'skipped')),
    completed_by        UUID        REFERENCES users(id) ON DELETE SET NULL,
    completed_at        TIMESTAMPTZ,
    completion_note     TEXT,
    attachment_url      TEXT,
    skip_reason         TEXT,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_pchk_item_completed CHECK (status <> 'completed' OR completed_at IS NOT NULL),
    CONSTRAINT chk_pchk_item_skipped   CHECK (status <> 'skipped'   OR skip_reason  IS NOT NULL)
);

CREATE INDEX idx_pchk_item_instance_id ON platform_checklist_instance_items (instance_id);
CREATE INDEX idx_pchk_item_assignee    ON platform_checklist_instance_items (assignee_user_id) WHERE assignee_user_id IS NOT NULL;
-- Deliberately a dead index for now — due_date is written (frozen at
-- instantiation) but read by nothing until reminders/Phase 9 land. Kept
-- because backfilling it onto a large table later is worse than an unused
-- index now.
CREATE INDEX idx_pchk_item_due ON platform_checklist_instance_items (due_date) WHERE due_date IS NOT NULL;

COMMENT ON TABLE  platform_checklist_instance_items IS 'Per-item runtime state for a checklist instance — a column-level snapshot, not JSONB, because items are individually completed/skipped and must be aggregated';
COMMENT ON COLUMN platform_checklist_instance_items.assignee_user_id IS 'NULL for role-owned group-claim items or unresolvable owners — fail-soft by design, see internal/hrm/onboarding';
COMMENT ON COLUMN platform_checklist_instance_items.due_date         IS 'Frozen at instantiation; not read by anything until reminders/Phase 9 (idx_pchk_item_due is a deliberate dead index until then)';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS platform_checklist_instance_items;
DROP TABLE IF EXISTS platform_checklist_instances;
DROP TABLE IF EXISTS platform_checklist_template_items;
DROP TABLE IF EXISTS platform_checklist_templates;

-- +goose StatementEnd
