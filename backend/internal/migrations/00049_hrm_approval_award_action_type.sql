-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00049_hrm_approval_award_action_type
--
-- Bug fix: 'award' was never added to the CHECK constraints on
--   hrm_approval_templates.action_type
--   hrm_approval_instances.entity_type
--
-- Group E1 (hrm_awards, migration 00043) supports an approval chain
-- via hrm_awards.approval_instance_id, and the Go service layer
-- (awards.Service.Submit) creates instances with entity_type='award'
-- and looks up templates via approvals.ActionTypeAward — but the
-- original migration 00024 (hrm_approval_chains) predates the awards
-- module and its CHECK constraints only ever listed the six action
-- types that existed at the time. Without this fix, any attempt to
-- create an award approval template or an award approval instance
-- fails at the database layer with a check-constraint violation,
-- even though the Go code compiles and runs fine.
-- ============================================================

ALTER TABLE hrm_approval_templates
    DROP CONSTRAINT IF EXISTS hrm_approval_templates_action_type_check,
    ADD CONSTRAINT hrm_approval_templates_action_type_check
        CHECK (action_type IN (
            'leave', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'custom'
        ));

ALTER TABLE hrm_approval_instances
    DROP CONSTRAINT IF EXISTS hrm_approval_instances_entity_type_check,
    ADD CONSTRAINT hrm_approval_instances_entity_type_check
        CHECK (entity_type IN (
            'leave_request', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'award', 'custom'
        ));

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE hrm_approval_templates
    DROP CONSTRAINT IF EXISTS hrm_approval_templates_action_type_check,
    ADD CONSTRAINT hrm_approval_templates_action_type_check
        CHECK (action_type IN (
            'leave', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'custom'
        ));

ALTER TABLE hrm_approval_instances
    DROP CONSTRAINT IF EXISTS hrm_approval_instances_entity_type_check,
    ADD CONSTRAINT hrm_approval_instances_entity_type_check
        CHECK (entity_type IN (
            'leave_request', 'resignation', 'promotion', 'transfer',
            'warning', 'document', 'termination',
            'attendance_regularization', 'custom'
        ));

-- +goose StatementEnd