-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00131_hrm_entity_scoping_indexes
--
-- Phase 11B-2: entity re-scoping. NO NEW TABLES AND NO NEW COLUMNS — two
-- indexes, and that is the whole schema change.
--
-- ⚠ THIS IS THE POINT OF PHASE 0.4. legal_entity_id was planted on 39 tables
-- long before anything read it, so the slice that finally reads it needs no
-- schema surgery at all. What it needs is for those reads to be indexed:
-- narrowing a payroll run to an entity's employees, and grouping an analytics
-- snapshot by entity, are both new access patterns against columns that have
-- only ever been written.
--
-- ⚠ hrm_employees.legal_entity_id had NO INDEX. Every entity-narrowed payroll
-- run would have been a sequential scan of the employee table, on the hot
-- path of the most performance-sensitive job in the product.
--
-- Both are partial (WHERE legal_entity_id IS NOT NULL) because the
-- overwhelming majority of rows are NULL — that is the single-entity default
-- and it is not going to change. Indexing the NULLs would be indexing almost
-- the whole table to find nothing.
-- ============================================================

CREATE INDEX idx_hrm_emp_legal_entity
    ON hrm_employees (org_id, legal_entity_id)
    WHERE legal_entity_id IS NOT NULL;

CREATE INDEX idx_hrm_payslip_runs_legal_entity
    ON hrm_payslip_runs (org_id, legal_entity_id)
    WHERE legal_entity_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_hrm_payslip_runs_legal_entity;
DROP INDEX IF EXISTS idx_hrm_emp_legal_entity;

-- +goose StatementEnd
