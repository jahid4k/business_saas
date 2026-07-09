-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00042_hrm_announcements
--
-- HRM org-wide and targeted announcements — Group E2.
-- Integrates with C4 (hrm_acknowledgements) when
-- requires_acknowledgement = TRUE.
--
-- Status machine:
--   draft → scheduled | published → expired | archived
--
-- Scope types:
--   organization  → everyone in the org (scope_ids ignored)
--   department    → scope_ids = array of department UUIDs
--   individual    → scope_ids = array of employee UUIDs
--
-- C4 integration:
--   When published AND requires_acknowledgement = TRUE, the service
--   creates one hrm_acknowledgements row per target employee with
--   acknowledgeable_type = 'announcement'.
--
-- E1 integration:
--   hrm_awards.announcement_id is back-filled here after both tables exist.
-- ============================================================

CREATE TABLE hrm_announcements (
    id                        UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id                 TEXT        NOT NULL UNIQUE
                                              DEFAULT ('ann_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                    UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    title                     TEXT        NOT NULL,
    content                   TEXT        NOT NULL,   -- Markdown supported

    category                  TEXT        NOT NULL DEFAULT 'general'
                                              CHECK (category IN (
                                                  'general', 'policy', 'event', 'award',
                                                  'reminder', 'emergency', 'hr_update'
                                              )),

    -- Who sees this announcement
    scope_type                TEXT        NOT NULL DEFAULT 'organization'
                                              CHECK (scope_type IN ('organization', 'department', 'individual')),
    scope_ids                 UUID[]      NOT NULL DEFAULT '{}',   -- empty = whole org

    -- Scheduling
    scheduled_at              TIMESTAMPTZ,      -- NULL = publish immediately on status change
    published_at              TIMESTAMPTZ,
    expires_at                TIMESTAMPTZ,

    -- C4 acknowledgement integration
    requires_acknowledgement  BOOLEAN     NOT NULL DEFAULT FALSE,
    acknowledgement_deadline  DATE,

    -- Visibility controls
    is_pinned                 BOOLEAN     NOT NULL DEFAULT FALSE,
    pin_order                 INTEGER     NOT NULL DEFAULT 0,

    -- Author
    author_id                 UUID        NOT NULL REFERENCES users(id),

    status                    TEXT        NOT NULL DEFAULT 'draft'
                                              CHECK (status IN (
                                                  'draft', 'scheduled', 'published',
                                                  'expired', 'archived'
                                              )),

    created_by                UUID        NOT NULL REFERENCES users(id),
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_ann_org_id    ON hrm_announcements (org_id);
CREATE INDEX idx_hrm_ann_status    ON hrm_announcements (org_id, status);
CREATE INDEX idx_hrm_ann_category  ON hrm_announcements (org_id, category);
CREATE INDEX idx_hrm_ann_pinned    ON hrm_announcements (org_id, is_pinned, pin_order) WHERE is_pinned = TRUE;
CREATE INDEX idx_hrm_ann_scheduled ON hrm_announcements (scheduled_at) WHERE status = 'scheduled';
CREATE INDEX idx_hrm_ann_expires   ON hrm_announcements (expires_at) WHERE expires_at IS NOT NULL AND status = 'published';

COMMENT ON TABLE  hrm_announcements IS 'Org, department, or individual-targeted announcements with optional C4 acknowledgement';
COMMENT ON COLUMN hrm_announcements.scope_ids             IS 'Dept UUIDs (scope=department) or employee UUIDs (scope=individual); empty for org-wide';
COMMENT ON COLUMN hrm_announcements.requires_acknowledgement IS 'When true, publishing auto-creates C4 acknowledgement requests for each target employee';

-- Back-fill FK on hrm_awards.announcement_id now that hrm_announcements exists
ALTER TABLE hrm_awards
    ADD CONSTRAINT fk_hrm_awd_announcement
    FOREIGN KEY (announcement_id) REFERENCES hrm_announcements(id) ON DELETE SET NULL;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE hrm_awards DROP CONSTRAINT IF EXISTS fk_hrm_awd_announcement;
DROP TABLE IF EXISTS hrm_announcements;

-- +goose StatementEnd
