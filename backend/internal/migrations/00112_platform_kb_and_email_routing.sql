-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00112_platform_kb_and_email_routing
--
-- Phase 8D, the final slice of Phase 8. Three unrelated-looking changes that
-- belong together because they are what "helpdesk, finished" requires:
--
--   1. platform_kb_categories / platform_kb_articles — the knowledge base
--   2. org_inbound_emails.destination — email-to-ticket routing
--   3. crm_leads.created_by DROP NOT NULL — a PRE-EXISTING defect fix,
--      without which (2) cannot be verified. See its section below.
--
-- ⚠ (2) MODIFIES A WORKING PRODUCTION PIPELINE. internal/capture/email's
-- ProcessInboundWebhook hardcoded lead creation: it was a single-consumer
-- pipeline, not a router. The whole risk of this slice is regressing lead
-- capture, so `destination` DEFAULTs to 'lead' and every existing row takes
-- that default — this migration alone changes no behaviour at all. The
-- receiving address decides: sales@ makes a lead, support@ makes a ticket.
--
-- What must NEVER be added (the 00076 CHECK x ON DELETE SET NULL trap):
-- platform_kb_articles.category_id is ON DELETE SET NULL, so
-- CHECK (status <> 'published' OR category_id IS NOT NULL) would make
-- DELETE FROM platform_kb_categories fail 23514 for any org holding a
-- published article. Postgres re-evaluates CHECKs on UPDATE and SET NULL
-- *is* an UPDATE. An uncategorised article is a valid state anyway.
-- ============================================================

-- ------------------------------------------------------------
-- platform_kb_categories
-- ------------------------------------------------------------
CREATE TABLE platform_kb_categories (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id   TEXT        NOT NULL UNIQUE
                                DEFAULT ('kbcat_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name        TEXT        NOT NULL,
    description TEXT,
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,

    created_by  UUID        NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX        idx_kbcat_org     ON platform_kb_categories (org_id, is_active);
CREATE UNIQUE INDEX uq_kbcat_org_name ON platform_kb_categories (org_id, LOWER(name));

-- ------------------------------------------------------------
-- platform_kb_articles
--
-- NOTE the absence of view_count. A counter nothing can recompute is
-- unauditable the moment it drifts, and deriving one would need a view-event
-- table nobody has asked for. The 00076 rule cuts both ways: don't store what
-- can be derived, and don't invent state no consumer needs.
-- ------------------------------------------------------------
CREATE TABLE platform_kb_articles (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id      TEXT        NOT NULL UNIQUE
                                   DEFAULT ('kba_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id         UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    category_id    UUID        REFERENCES platform_kb_categories(id) ON DELETE SET NULL,

    title          TEXT        NOT NULL,
    body           TEXT        NOT NULL,
    -- Draft articles are visible only to platform.kb.manage holders. A
    -- half-written HR policy read as authoritative is worse than no article.
    status         TEXT        NOT NULL DEFAULT 'draft'
                                   CHECK (status IN ('draft', 'published', 'archived')),

    author_user_id UUID        NOT NULL REFERENCES users(id),
    published_at   TIMESTAMPTZ,

    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_kba_org_status ON platform_kb_articles (org_id, status);
CREATE INDEX idx_kba_category   ON platform_kb_articles (category_id) WHERE category_id IS NOT NULL;

-- Full-text search over title and body. An EXPRESSION index, so there is no
-- stored search_vector column to keep in sync with the text it indexes —
-- Postgres recomputes it on write, which is the one place a derived value
-- genuinely cannot drift. A knowledge base nobody can search is a filing
-- cabinet with no labels.
CREATE INDEX idx_kba_search ON platform_kb_articles
    USING GIN (to_tsvector('english', title || ' ' || body));

COMMENT ON TABLE platform_kb_articles IS 'Knowledge base articles. No view_count column by design — a counter nothing can recompute is unauditable, and no consumer needs one.';

-- ------------------------------------------------------------
-- org_inbound_emails.destination — the router
-- ------------------------------------------------------------
-- DEFAULT 'lead' and NOT NULL together are what make this migration a no-op
-- for existing installs: every row that exists keeps doing exactly what it
-- did. The ticket branch is purely additive.
ALTER TABLE org_inbound_emails
    ADD COLUMN destination TEXT NOT NULL DEFAULT 'lead'
        CHECK (destination IN ('lead', 'ticket'));

COMMENT ON COLUMN org_inbound_emails.destination IS 'Where an inbound email to this address goes. Defaults to lead so pre-existing addresses are unaffected.';

-- ------------------------------------------------------------
-- crm_leads.created_by — PRE-EXISTING DEFECT FIX
-- ------------------------------------------------------------
-- Every system-originated lead capture in this codebase was failing.
-- internal/capture/{email,social,visitors} all call
-- leads.CreateLead(ctx, orgID, "", req) — an empty userID — and created_by
-- was uuid NOT NULL, so each insert died with
--   invalid input syntax for type uuid: "" (SQLSTATE 22P02)
-- and the error was swallowed into inbound_email_logs.error_message while
-- the webhook still returned 200. Silent, and it had been silent since the
-- capture modules were built.
--
-- Found while writing the "lead capture still works" regression test this
-- slice requires, BEFORE any routing change — the test could not be made
-- green against the existing code, which is the whole reason it was written
-- first.
--
-- NOT NULL was the assertion that lied: a system-captured lead genuinely has
-- no human creator, and capture_source already records where it came from.
-- Attributing it to the org owner instead would put a person's name on an
-- action they did not take, in an audit column.
ALTER TABLE crm_leads ALTER COLUMN created_by DROP NOT NULL;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Articles reference categories, so articles go first. Not the mirror of
-- CREATE order — 6B's 00094 shipped a broken Down block assuming symmetry.
DROP TABLE IF EXISTS platform_kb_articles;
DROP TABLE IF EXISTS platform_kb_categories;

ALTER TABLE org_inbound_emails DROP COLUMN IF EXISTS destination;

-- Restoring NOT NULL would fail against any row written while it was
-- nullable — which is every system-captured lead, i.e. exactly the rows this
-- migration exists to permit. Null them out first so the down block is
-- genuinely runnable rather than theoretically correct: those leads were
-- impossible to create before this migration, so deleting them restores the
-- prior state honestly.
DELETE FROM crm_leads WHERE created_by IS NULL;
ALTER TABLE crm_leads ALTER COLUMN created_by SET NOT NULL;

-- +goose StatementEnd
