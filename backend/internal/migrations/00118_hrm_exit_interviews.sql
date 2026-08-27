-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00118_hrm_exit_interviews
--
-- Phase 9C, the final slice of Phase 9. One table plus two CHECK widenings:
--
--   hrm_exit_interviews  — the exit interview, on the Phase 5 form engine
--   document_type        — 'relieving_letter' added to BOTH constraints
--
-- ⚠ THE INTERVIEW IS CONFIDENTIAL, AND CONFIDENTIALITY IS STRUCTURAL.
-- A departing employee answers honestly only if their answers cannot reach
-- the manager they are leaving. The responses live in platform_form_instances
-- (this table holds only the link and the lifecycle), and the service exposes
-- them through a read path gated on a permission the manager does not hold —
-- the 5C 360-feedback shape, where anonymity is enforced by which query runs
-- rather than by a field somebody remembers to strip.
--
-- Design notes:
--
--   • SENT POST-DEPARTURE, via the scheduler. An interview sent while the
--     employee is still on the payroll gets a different answer from one sent
--     after they have left, and the whole point is the honest one.
--     scheduled_for is the earliest date the sweep may send it — normally
--     the day after last_working_date.
--
--   • form_instance_id is NULLABLE and set when the interview is actually
--     sent. An interview row therefore exists BEFORE its form does, which is
--     what lets HR schedule one for an org that has not configured a
--     template yet without the row silently not existing.
--
--   • platform_form_instances.form_type already permits 'exit_interview',
--     in both the Go enum (forms/model.go) and the DB CHECK (00084) — built
--     ahead of this consumer, and verified to have NOT drifted apart the way
--     AckType did for two phases (see r29).
--
--   • THERE IS NO responses/answers COLUMN HERE. The form engine owns the
--     responses; duplicating them would create a second copy that disagrees
--     with the first the moment one is edited. The 00076 rule.
--
-- What must NEVER be added (the 00076 CHECK x ON DELETE SET NULL trap):
-- form_instance_id is ON DELETE SET NULL, so
--   CHECK (status <> 'sent' OR form_instance_id IS NOT NULL)
-- would make DELETE FROM platform_form_instances fail 23514 for any sent
-- interview. The service validates the pairing instead.
-- ============================================================

CREATE TABLE hrm_exit_interviews (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id         TEXT        NOT NULL UNIQUE
                                      DEFAULT ('exiv_' || REPLACE(gen_random_uuid()::TEXT, '-', '')),
    org_id            UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    exit_id           UUID        NOT NULL REFERENCES hrm_exits(id) ON DELETE CASCADE,

    -- The form carrying the actual answers. NULL until the interview is sent;
    -- see the migration header.
    form_instance_id  UUID        REFERENCES platform_form_instances(id) ON DELETE SET NULL,

    status            TEXT        NOT NULL DEFAULT 'scheduled'
                                      CHECK (status IN (
                                          'scheduled', 'sent', 'completed', 'declined', 'cancelled'
                                      )),

    -- The earliest date the scheduler may send this. Normally the day after
    -- last_working_date: an interview answered while still on the payroll is
    -- not the honest one.
    scheduled_for     DATE        NOT NULL,
    sent_at           TIMESTAMPTZ,
    completed_at      TIMESTAMPTZ,

    created_by        UUID        NOT NULL REFERENCES users(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One interview per exit. A second would split the answers across two
-- records and make any aggregate double-count one person.
CREATE UNIQUE INDEX uq_hrm_exiv_exit ON hrm_exit_interviews (exit_id);
CREATE INDEX idx_hrm_exiv_org_status ON hrm_exit_interviews (org_id, status);
-- Backs the send sweep: scheduled interviews whose date has arrived.
CREATE INDEX idx_hrm_exiv_due ON hrm_exit_interviews (scheduled_for)
    WHERE status = 'scheduled';

COMMENT ON TABLE hrm_exit_interviews IS 'Exit interview lifecycle. Responses live in platform_form_instances — this table deliberately stores none, so there is only ever one copy. Confidential: individual responses are readable only through a permission-gated service path, the 5C 360-feedback shape.';
COMMENT ON COLUMN hrm_exit_interviews.scheduled_for IS 'Earliest date the scheduler may send. Normally the day AFTER last_working_date — an interview answered while still employed is not the honest one.';

-- ------------------------------------------------------------
-- document_type: add 'relieving_letter' to BOTH constraints
-- ------------------------------------------------------------
-- ⚠ THE TWO-CHECK WIDENING TRAP, FOURTH OCCURRENCE.
-- hrm_document_templates.document_type and hrm_employee_documents.document_type
-- are SEPARATE constraints with DIFFERENT vocabularies (00026 lines 35 and
-- 85): the template list has 11 values, the document list has those plus
-- 'passport', 'visa', 'certificate' and 'id_proof'. Widening only one leaves
-- a template that can be created but never issued, or vice versa. This bit
-- 7B, was re-hit in 7C, and recurred in 8A/8B with the approval CHECKs.
--
-- The Go DocumentType enum AND its IsValid() switch are widened alongside —
-- 8A found AckType had drifted from its own CHECK for two phases because
-- exactly that step was skipped, leaving two DB-legal values unreachable
-- through the only typed write path.
--
-- 'experience_letter' already exists in both and needs nothing.
ALTER TABLE hrm_document_templates DROP CONSTRAINT hrm_document_templates_document_type_check;
ALTER TABLE hrm_document_templates ADD CONSTRAINT hrm_document_templates_document_type_check
    CHECK (document_type IN (
        'offer_letter', 'contract', 'warning_letter',
        'promotion_letter', 'transfer_letter',
        'termination_letter', 'resignation_acceptance',
        'experience_letter', 'relieving_letter', 'nda', 'policy', 'custom'
    ));

ALTER TABLE hrm_employee_documents DROP CONSTRAINT hrm_employee_documents_document_type_check;
ALTER TABLE hrm_employee_documents ADD CONSTRAINT hrm_employee_documents_document_type_check
    CHECK (document_type IN (
        'offer_letter', 'contract', 'warning_letter',
        'promotion_letter', 'transfer_letter',
        'termination_letter', 'resignation_acceptance',
        'experience_letter', 'relieving_letter', 'nda', 'policy',
        'passport', 'visa', 'certificate',
        'id_proof', 'custom'
    ));

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hrm_exit_interviews;

-- Restore BOTH constraints to their EXACT 00026 state — not a merged or
-- "tidied" version. Over-reverting is as much a defect as under-reverting:
-- the two lists differ deliberately and must keep differing.
ALTER TABLE hrm_document_templates DROP CONSTRAINT hrm_document_templates_document_type_check;
ALTER TABLE hrm_document_templates ADD CONSTRAINT hrm_document_templates_document_type_check
    CHECK (document_type IN (
        'offer_letter', 'contract', 'warning_letter',
        'promotion_letter', 'transfer_letter',
        'termination_letter', 'resignation_acceptance',
        'experience_letter', 'nda', 'policy', 'custom'
    ));

ALTER TABLE hrm_employee_documents DROP CONSTRAINT hrm_employee_documents_document_type_check;
ALTER TABLE hrm_employee_documents ADD CONSTRAINT hrm_employee_documents_document_type_check
    CHECK (document_type IN (
        'offer_letter', 'contract', 'warning_letter',
        'promotion_letter', 'transfer_letter',
        'termination_letter', 'resignation_acceptance',
        'experience_letter', 'nda', 'policy',
        'passport', 'visa', 'certificate',
        'id_proof', 'custom'
    ));

-- +goose StatementEnd
