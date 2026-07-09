-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00024_hrm_document_templates
--
-- HRM document system (Group A4):
--   hrm_document_templates — Markdown templates with {{placeholder}} syntax
--   hrm_employee_documents — instances: generated from template OR direct upload
--   hrm_document_bulk_sends — tracks org-wide / dept-wide send operations
--
-- Design notes:
--   • template_id is nullable on hrm_employee_documents — null means direct upload.
--   • body_markdown stores the template text with {{employee.first_name}} style vars.
--   • generated_content stores the resolved HTML/Markdown (filled placeholders).
--     browser renders/prints to PDF — no server-side PDF generation dependency.
--   • related_type / related_id is polymorphic: links doc to warning, promotion, etc.
--   • version increments when a doc is superseded; superseded_by links to newer row.
--   • status machine: draft → sent → acknowledged / declined / expired / withdrawn
--   • This migration also back-fills the FK on hrm_warning_types.document_template_id
--     which was intentionally left NULL in migration 00023 (chicken-and-egg).
-- ============================================================

-- ------------------------------------------------------------
-- hrm_document_templates
-- ------------------------------------------------------------
CREATE TABLE hrm_document_templates (
    id                      UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id               TEXT    NOT NULL UNIQUE
                                        DEFAULT ('dt_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id                  UUID    NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    name                    TEXT    NOT NULL,
    document_type           TEXT    NOT NULL
                                        CHECK (document_type IN (
                                            'offer_letter', 'contract', 'warning_letter',
                                            'promotion_letter', 'transfer_letter',
                                            'termination_letter', 'resignation_acceptance',
                                            'experience_letter', 'nda', 'policy', 'custom'
                                        )),
    description             TEXT,

    -- Markdown body with {{placeholder}} syntax
    -- Available vars documented in available_variables TEXT[]
    body_markdown           TEXT    NOT NULL DEFAULT '',
    available_variables     TEXT[]  NOT NULL DEFAULT '{}',

    requires_acknowledgement BOOLEAN NOT NULL DEFAULT FALSE,
    is_active               BOOLEAN NOT NULL DEFAULT TRUE,

    created_by              UUID    NOT NULL REFERENCES users(id),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_dt_org_id ON hrm_document_templates (org_id);
CREATE UNIQUE INDEX idx_hrm_dt_org_name ON hrm_document_templates (org_id, LOWER(name)) WHERE is_active = TRUE;

COMMENT ON TABLE  hrm_document_templates IS 'Markdown document templates with placeholder variables';
COMMENT ON COLUMN hrm_document_templates.body_markdown        IS 'Template body; {{employee.first_name}} style placeholders';
COMMENT ON COLUMN hrm_document_templates.available_variables  IS 'List of supported variable paths for this template';

-- Now add the FK that migration 00023 left as a plain UUID column
ALTER TABLE hrm_warning_types
    ADD CONSTRAINT fk_hrm_wt_doc_template
    FOREIGN KEY (document_template_id)
    REFERENCES hrm_document_templates(id)
    ON DELETE SET NULL;

-- ------------------------------------------------------------
-- hrm_employee_documents
-- ------------------------------------------------------------
CREATE TABLE hrm_employee_documents (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id           TEXT        NOT NULL UNIQUE
                                        DEFAULT ('ed_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id              UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id         UUID        NOT NULL REFERENCES hrm_employees(id) ON DELETE CASCADE,

    -- Nullable: null = direct upload; set = generated from template
    template_id         UUID        REFERENCES hrm_document_templates(id) ON DELETE SET NULL,

    title               TEXT        NOT NULL,
    document_type       TEXT        NOT NULL
                                        CHECK (document_type IN (
                                            'offer_letter', 'contract', 'warning_letter',
                                            'promotion_letter', 'transfer_letter',
                                            'termination_letter', 'resignation_acceptance',
                                            'experience_letter', 'nda', 'policy',
                                            'passport', 'visa', 'certificate',
                                            'id_proof', 'custom'
                                        )),

    -- File storage (always populated — either uploaded or generated-then-stored)
    file_url            TEXT        NOT NULL,
    file_name           TEXT        NOT NULL,
    file_size_bytes     BIGINT,
    mime_type           TEXT        NOT NULL DEFAULT 'application/pdf',

    -- Generated content (null for direct uploads)
    generated_content   TEXT,       -- resolved Markdown/HTML before PDF render

    -- Polymorphic link to the entity that produced this document
    related_type        TEXT        CHECK (related_type IN (
                                        'warning', 'promotion', 'transfer',
                                        'resignation', 'termination', 'custom'
                                    )),
    related_id          UUID,

    -- Versioning (supersede rather than update)
    version             INTEGER     NOT NULL DEFAULT 1,
    superseded_by       UUID        REFERENCES hrm_employee_documents(id) ON DELETE SET NULL,

    -- Bulk send tracking
    bulk_send_batch_id  UUID,

    -- Expiry tracking (visas, certifications, contracts)
    expiry_date         DATE,

    status              TEXT        NOT NULL DEFAULT 'draft'
                                        CHECK (status IN (
                                            'draft', 'sent', 'acknowledged',
                                            'declined', 'expired', 'withdrawn', 'superseded'
                                        )),

    issued_by           UUID        REFERENCES users(id) ON DELETE SET NULL,
    sent_at             TIMESTAMPTZ,
    acknowledged_at     TIMESTAMPTZ,
    acknowledgement_note TEXT,

    created_by          UUID        NOT NULL REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_ed_org_id      ON hrm_employee_documents (org_id);
CREATE INDEX idx_hrm_ed_employee_id ON hrm_employee_documents (employee_id);
CREATE INDEX idx_hrm_ed_template_id ON hrm_employee_documents (template_id) WHERE template_id IS NOT NULL;
CREATE INDEX idx_hrm_ed_related     ON hrm_employee_documents (related_type, related_id) WHERE related_type IS NOT NULL;
CREATE INDEX idx_hrm_ed_status      ON hrm_employee_documents (status);
CREATE INDEX idx_hrm_ed_expiry      ON hrm_employee_documents (expiry_date) WHERE expiry_date IS NOT NULL;
CREATE INDEX idx_hrm_ed_batch_id    ON hrm_employee_documents (bulk_send_batch_id) WHERE bulk_send_batch_id IS NOT NULL;

COMMENT ON TABLE  hrm_employee_documents IS 'Employee document instances: generated from template or directly uploaded';
COMMENT ON COLUMN hrm_employee_documents.template_id      IS 'NULL for direct uploads; set for template-generated documents';
COMMENT ON COLUMN hrm_employee_documents.generated_content IS 'Resolved template content before PDF render; NULL for uploads';
COMMENT ON COLUMN hrm_employee_documents.version          IS 'Increments on supersede; links via superseded_by chain';

-- ------------------------------------------------------------
-- hrm_document_bulk_sends
-- ------------------------------------------------------------
CREATE TABLE hrm_document_bulk_sends (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id       TEXT        NOT NULL UNIQUE
                                    DEFAULT ('dbs_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id          UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    template_id     UUID        NOT NULL REFERENCES hrm_document_templates(id) ON DELETE RESTRICT,

    sender_id       UUID        NOT NULL REFERENCES users(id),

    recipient_type  TEXT        NOT NULL CHECK (recipient_type IN ('all', 'department', 'employee_list')),
    recipient_ids   JSONB       NOT NULL DEFAULT '[]',  -- UUID[] of dept_ids or employee_ids

    batch_id        UUID        NOT NULL DEFAULT gen_random_uuid(),  -- links to hrm_employee_documents.bulk_send_batch_id

    total_count     INTEGER     NOT NULL DEFAULT 0,
    pending_count   INTEGER     NOT NULL DEFAULT 0,
    completed_count INTEGER     NOT NULL DEFAULT 0,

    sent_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hrm_dbs_org_id      ON hrm_document_bulk_sends (org_id);
CREATE INDEX idx_hrm_dbs_template_id ON hrm_document_bulk_sends (template_id);
CREATE INDEX idx_hrm_dbs_batch_id    ON hrm_document_bulk_sends (batch_id);

COMMENT ON TABLE hrm_document_bulk_sends IS 'Tracks org-wide or dept-wide document send operations';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE hrm_warning_types DROP CONSTRAINT IF EXISTS fk_hrm_wt_doc_template;

DROP TABLE IF EXISTS hrm_document_bulk_sends;
DROP TABLE IF EXISTS hrm_employee_documents;
DROP TABLE IF EXISTS hrm_document_templates;

-- +goose StatementEnd
