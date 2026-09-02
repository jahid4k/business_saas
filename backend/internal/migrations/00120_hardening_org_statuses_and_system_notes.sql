-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00120_hardening_org_statuses_and_system_notes
--
-- A HARDENING PASS, not a feature. Two unrelated one-line schema changes that
-- travel together because they are both long-carried defects being cleared at
-- once — the same reason 00112 combined the KB, email routing and the
-- crm_leads.created_by fix. Neither introduces a new permission, so there is
-- no 00121 seed pair.
--
--   1. hrm_employee_statuses backfill — a fresh org cannot create an employee
--   2. platform_notes.created_by     — system-originated notes fail silently
--
-- ------------------------------------------------------------
-- 1. Employee statuses for organizations that have none
-- ------------------------------------------------------------
-- Migration 00053 seeded five statuses PER ORG, but only for the orgs that
-- existed when it ran — organizations.Create has never seeded them. So every
-- org created through the API since then has zero statuses, and
-- POST /hrm/employees fails on a NOT NULL status_id. This has been in the
-- known-open list since r18 and has been worked around by hand in essentially
-- every smoke run this project has done.
--
-- The Go side is fixed alongside this (organizations.Create now seeds inside
-- its existing transaction). This block repairs the orgs already created,
-- because without it the bug persists for all of them.
--
-- The five rows, their categories and their colors mirror 00053 EXACTLY, so a
-- migration-seeded org and an API-created org are indistinguishable. Two rows
-- share the 'terminated' category deliberately: 'Resigned' and 'Terminated'
-- are different names for the same lifecycle state, and HRM code filters on
-- CATEGORY, never on name — names are org-customisable, categories are
-- CHECK-constrained. Getting the category right is what makes payroll's
-- eligible-employee filter work for a new org.
INSERT INTO hrm_employee_statuses (org_id, name, category, color)
SELECT o.id, v.name, v.category, v.color
  FROM organizations o
 CROSS JOIN (VALUES
        ('Active',     'active',     'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'),
        ('Inactive',   'inactive',   'bg-zinc-500/10 text-zinc-400 border-zinc-500/20'),
        ('On Leave',   'on_leave',   'bg-amber-500/10 text-amber-400 border-amber-500/20'),
        ('Resigned',   'terminated', 'bg-orange-500/10 text-orange-400 border-orange-500/20'),
        ('Terminated', 'terminated', 'bg-red-500/10 text-red-400 border-red-500/20')
      ) AS v(name, category, color)
 WHERE NOT EXISTS (
     SELECT 1 FROM hrm_employee_statuses s WHERE s.org_id = o.id
 );

-- ------------------------------------------------------------
-- 2. platform_notes.created_by — the sibling 00112 left unfixed
-- ------------------------------------------------------------
-- 00112 made crm_leads.created_by nullable because every internal/capture/*
-- path creates leads with no acting user. The identical problem sits one call
-- deeper: leads.CreateLead's duplicate-capture path calls
-- engagement.CreateNote(ctx, orgID, userID, ...) with that same empty userID,
-- created_by is NOT NULL REFERENCES users(id) (00015), and the error is
-- discarded with `_, _ =`. So a repeat inbound email from a known sender
-- silently fails to record its duplicate-capture note.
--
-- NOT NULL is the assertion that lies: a system-generated note genuinely has
-- no human author.
--
-- ⚠ platform_tasks, platform_activities and platform_email_logs are
-- DELIBERATELY LEFT ALONE. Their created_by is also NOT NULL, but nothing
-- creates them without an actor — crm/deals passes a real author id. Widening
-- a constraint with no consumer is the "built ahead of its consumer"
-- antipattern this codebase has now hit four times; when one of those paths
-- gains a system caller, that is the moment to widen it.
ALTER TABLE platform_notes ALTER COLUMN created_by DROP NOT NULL;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Restoring NOT NULL fails against any row written while it was nullable —
-- which is every system-generated note, i.e. exactly the rows this migration
-- exists to permit. Delete them first so the down block is genuinely runnable
-- rather than theoretically correct; they were impossible to create before.
-- Same treatment 00112 gave crm_leads.
DELETE FROM platform_notes WHERE created_by IS NULL;
ALTER TABLE platform_notes ALTER COLUMN created_by SET NOT NULL;

-- The status backfill is NOT reverted.
--
-- There is no way to distinguish a row this migration inserted from one an org
-- has since renamed, recoloured or come to depend on — and deleting an
-- employee's status would orphan hrm_employees.status_id, which is NOT NULL.
-- Reverting would therefore risk destroying live data to undo a repair. An
-- org having the same five statuses it would have had all along is not a
-- state anything needs rolled back.

-- +goose StatementEnd
