-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00110_platform_tickets
--
-- Phase 8C: the ticket engine. Five tables:
--
--   platform_ticket_categories  — org-defined categories, some SENSITIVE
--   platform_sla_policies       — response/resolution targets per priority
--   platform_tickets            — the ticket itself
--   platform_ticket_comments    — public and INTERNAL-ONLY comments
--   platform_ticket_sla_events  — append-only pause/resume ledger
--
-- ⚠ THIS IS A PLATFORM PRIMITIVE. It must never reference hrm_* and the Go
-- package must never import internal/hrm/scope. That is not stylistic: the
-- build plan flagged HR Helpdesk vs the customer-facing Ticketing/Helpdesk in
-- the CRM list as "an architectural fork to decide before starting", and this
-- resolves it toward platform. The decision is already made twice elsewhere in
-- this codebase — platform_checklist_instances.subject_type CHECK IN
-- ('employee') and platform_form_instances.subject_type CHECK IN
-- ('employee','candidate') are both deliberately narrow polymorphic
-- discriminators, widened later. requester_type is the third instance of that
-- pattern, not a new one.
--
-- Design notes:
--
--   • requester_type starts as CHECK IN ('employee') with a NO-FK
--     requester_id. When customer-facing ticketing lands, adding 'contact' is
--     a CHECK widening, not a rewrite — and no hrm_* FK has to be untangled
--     first, because there never was one.
--
--   • THE SLA CLOCK IS PAUSABLE, AND PAUSES LIVE IN AN APPEND-ONLY LEDGER,
--     not a mutable paused_duration counter on the ticket. One ticket can be
--     paused and resumed several times ("waiting on the requester", twice),
--     and a single counter cannot be audited — you can see the number but
--     never how it got there. Same reasoning as 7C's
--     hrm_loan_recovery_events, where one installment recovered across three
--     runs needed three rows. Elapsed SLA time is computed from the ledger on
--     every read (the 00076 rule); there is deliberately no elapsed_minutes
--     or paused_minutes column, and an integration test introspects
--     information_schema to prove it.
--
--   • is_internal comments are filtered AT THE REPOSITORY LAYER — two
--     separate read methods, so the requester's path has no internal comment
--     in scope to forget to strip. Structural, not disciplinary: the exact
--     shape 5C used for 360-feedback anonymity and 6A for quiz answer keys.
--     A boolean the handler is trusted to check is the version of this that
--     eventually leaks.
--
--   • SENSITIVE CATEGORIES carry a restricted_role. A ticket in a sensitive
--     category may only be assigned to a holder of that role — harassment
--     complaints do not go into the general helpdesk queue. The role is
--     stored as a NAME rather than an FK because roles are org-scoped and
--     partly system-seeded; the service resolves it via the role directory,
--     the platform/checklists owner_type='role' precedent.
--
--   • converted_to_type / converted_to_id record the ONE-WAY conversion of a
--     ticket into something with more weight (an HR complaint). Polymorphic
--     and FK-free for the same reason requester_id is: platform must not
--     reference hrm. The conversion is initiated from the HRM side, which
--     reads the ticket and calls back to mark it — hrm → platform is the
--     allowed direction. One-way is deliberate: a formal complaint carries
--     legal weight and must never degrade back into a ticket.
--
-- What must NEVER be added (the 00076 CHECK x ON DELETE SET NULL trap):
-- Postgres re-evaluates CHECKs on UPDATE and ON DELETE SET NULL *is* an
-- UPDATE. assignee_user_id, sla_policy_id and category_id are all ON DELETE
-- SET NULL, so:
--   • CHECK (status <> 'assigned' OR assignee_user_id IS NOT NULL) would make
--     DELETE FROM users fail 23514 for any org holding an assigned ticket.
--   • CHECK (converted_to_id IS NOT NULL OR status <> 'converted') pairs two
--     nullable columns and would fail the same way.
-- The service validates both pairings instead.
-- ============================================================

-- ------------------------------------------------------------
-- platform_ticket_categories
-- ------------------------------------------------------------
CREATE TABLE platform_ticket_categories (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT        NOT NULL UNIQUE
                                    DEFAULT ('tcat_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id          UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name            TEXT        NOT NULL,
    description     TEXT,
    -- A sensitive category restricts who may be assigned its tickets.
    is_sensitive    BOOLEAN     NOT NULL DEFAULT FALSE,
    -- Role NAME, not an FK — roles are org-scoped and partly system-seeded,
    -- the platform/checklists owner_type='role' precedent. Only meaningful
    -- when is_sensitive; the service enforces that pairing rather than a
    -- CHECK, which would be the 00076 trap if either column ever gained an
    -- ON DELETE SET NULL FK.
    restricted_role TEXT,

    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_by      UUID        NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX        idx_ptcat_org_id  ON platform_ticket_categories (org_id, is_active);
CREATE UNIQUE INDEX uq_ptcat_org_name ON platform_ticket_categories (org_id, LOWER(name));

COMMENT ON COLUMN platform_ticket_categories.restricted_role IS 'Role NAME whose holders may be assigned tickets in this category. Only meaningful when is_sensitive — a harassment queue is not the general helpdesk queue.';

-- ------------------------------------------------------------
-- platform_sla_policies
-- ------------------------------------------------------------
CREATE TABLE platform_sla_policies (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id               TEXT        NOT NULL UNIQUE
                                            DEFAULT ('sla_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                  UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    -- NULL category_id is the org-wide default for that priority. A
    -- category-specific policy wins; resolution order lives in the service,
    -- the hrm_per_diem_rates (00108) shape.
    category_id             UUID        REFERENCES platform_ticket_categories(id) ON DELETE CASCADE,

    priority                TEXT        NOT NULL
                                            CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    first_response_minutes  INTEGER     NOT NULL CHECK (first_response_minutes > 0),
    resolution_minutes      INTEGER     NOT NULL CHECK (resolution_minutes > 0),

    created_by              UUID        NOT NULL REFERENCES users(id),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_psla_resolution_after_response
        CHECK (resolution_minutes >= first_response_minutes)
);

CREATE INDEX idx_psla_lookup ON platform_sla_policies (org_id, priority, category_id NULLS LAST);

-- ------------------------------------------------------------
-- platform_tickets
--
-- NOTE the absence of elapsed_minutes / paused_minutes / sla_breached. All
-- three are computed from platform_ticket_sla_events — see migration header.
-- ------------------------------------------------------------
CREATE TABLE platform_tickets (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id         TEXT        NOT NULL UNIQUE
                                      DEFAULT ('tkt_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id            UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Polymorphic and FK-FREE, so this table never references hrm_*.
    -- Deliberately narrow today; widening to 'contact' when customer-facing
    -- ticketing lands is a CHECK change, not a rewrite.
    requester_type    TEXT        NOT NULL DEFAULT 'employee'
                                      CHECK (requester_type IN ('employee')),
    requester_id      UUID        NOT NULL,
    -- The platform user who raised it, for "my tickets" and comment authorship.
    requester_user_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    category_id       UUID        REFERENCES platform_ticket_categories(id) ON DELETE SET NULL,
    subject           TEXT        NOT NULL,
    description       TEXT,
    priority          TEXT        NOT NULL DEFAULT 'normal'
                                      CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    status            TEXT        NOT NULL DEFAULT 'open'
                                      CHECK (status IN (
                                          'open', 'assigned', 'paused', 'resolved',
                                          'closed', 'converted', 'cancelled'
                                      )),

    assignee_user_id  UUID        REFERENCES users(id) ON DELETE SET NULL,
    sla_policy_id     UUID        REFERENCES platform_sla_policies(id) ON DELETE SET NULL,

    -- Stamped when they happen; the DUE times are computed from the policy
    -- plus the pause ledger, never stored.
    first_response_at TIMESTAMPTZ,
    resolved_at       TIMESTAMPTZ,
    closed_at         TIMESTAMPTZ,

    -- One-way conversion target. Polymorphic and FK-free — platform must not
    -- reference hrm. Set by the HRM side calling back in.
    converted_to_type TEXT        CHECK (converted_to_type IS NULL OR converted_to_type IN ('complaint')),
    converted_to_id   UUID,
    converted_at      TIMESTAMPTZ,

    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ptkt_org_status ON platform_tickets (org_id, status);
CREATE INDEX idx_ptkt_requester  ON platform_tickets (org_id, requester_type, requester_id);
CREATE INDEX idx_ptkt_assignee   ON platform_tickets (assignee_user_id) WHERE assignee_user_id IS NOT NULL;
CREATE INDEX idx_ptkt_open       ON platform_tickets (org_id, priority)
    WHERE status IN ('open', 'assigned', 'paused');

COMMENT ON TABLE platform_tickets IS 'A ticket. requester_type/requester_id are polymorphic and FK-free so this platform table never references hrm_*. There is deliberately NO elapsed_minutes, paused_minutes or sla_breached column — all are computed from platform_ticket_sla_events.';

-- ------------------------------------------------------------
-- platform_ticket_comments
-- ------------------------------------------------------------
CREATE TABLE platform_ticket_comments (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id      TEXT        NOT NULL UNIQUE
                                   DEFAULT ('tcmt_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    ticket_id      UUID        NOT NULL REFERENCES platform_tickets(id) ON DELETE CASCADE,
    author_user_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    body           TEXT        NOT NULL,
    -- An internal comment is agent-to-agent and the requester must never see
    -- it. Filtered at the REPOSITORY layer via two separate read methods —
    -- see migration header.
    is_internal    BOOLEAN     NOT NULL DEFAULT FALSE,

    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ptcmt_ticket ON platform_ticket_comments (ticket_id, created_at);
-- Backs the requester's read path specifically, so the common case never
-- scans internal rows at all.
CREATE INDEX idx_ptcmt_public ON platform_ticket_comments (ticket_id, created_at)
    WHERE is_internal = FALSE;

-- ------------------------------------------------------------
-- platform_ticket_sla_events — append-only pause/resume ledger
-- ------------------------------------------------------------
CREATE TABLE platform_ticket_sla_events (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id   UUID        NOT NULL REFERENCES platform_tickets(id) ON DELETE CASCADE,

    event_type  TEXT        NOT NULL CHECK (event_type IN ('pause', 'resume')),
    reason      TEXT,
    actor_id    UUID        REFERENCES users(id) ON DELETE SET NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    -- No updated_at — append-only. Correcting a mistake means adding an
    -- event, never editing one, exactly like hrm_loan_recovery_events and
    -- hrm_application_stage_history.
);

CREATE INDEX idx_ptsla_ticket ON platform_ticket_sla_events (ticket_id, occurred_at);

COMMENT ON TABLE platform_ticket_sla_events IS 'Append-only pause/resume ledger. Paused time is SUM over resume-pause pairs, computed on read — a mutable paused_minutes counter would show the number but never how it got there.';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Reverse dependency order: events and comments reference tickets; tickets
-- reference categories AND sla policies; sla policies reference categories.
-- So categories must go LAST, and policies after tickets. Not the mirror of
-- CREATE order — 6B's 00094 shipped a broken Down block assuming symmetry.
DROP TABLE IF EXISTS platform_ticket_sla_events;
DROP TABLE IF EXISTS platform_ticket_comments;
DROP TABLE IF EXISTS platform_tickets;
DROP TABLE IF EXISTS platform_sla_policies;
DROP TABLE IF EXISTS platform_ticket_categories;

-- +goose StatementEnd
