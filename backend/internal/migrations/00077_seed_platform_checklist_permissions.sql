-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00077_seed_platform_checklist_permissions
--
-- Phase 3: three new keys on the platform.checklists resource:
--   platform.checklists.view     — read templates/instances/items
--   platform.checklists.complete — mark an item complete/reopen it
--   platform.checklists.manage   — template CRUD, item skip, instance cancel
--
-- .complete is granted broadly (through member) then NARROWED by the
-- service at the individual-item level (assignee, or role holder, or
-- .manage) — the route gate cannot express "is this your own item", so
-- it deliberately does not try. viewer is excluded from .complete
-- (read-only by definition).
--
-- .manage is owner/admin only — the same line 00075 drew for
-- hrm.leave.adjust_balance. Consequence accepted knowingly: a manager
-- cannot skip or cancel a checklist item in Phase 3. Loosening later is
-- a one-line migration; tightening later is a support incident.
--
-- Unlike 00073's view_own backfill (which RESTORED prior behaviour),
-- these are brand-new capabilities. Org-created custom roles get none of
-- them until an admin grants them explicitly — this is deliberate, not
-- an oversight, and is NOT backfilled onto custom roles the way 00073
-- backfilled view_own.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('platform.checklists.view',     'platform.checklists', 'view',     'View checklist templates, instances, and items'),
    ('platform.checklists.complete', 'platform.checklists', 'complete', 'Complete, reopen, or view items assigned to the caller'),
    ('platform.checklists.manage',   'platform.checklists', 'manage',   'Manage checklist templates and administer instances/items')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY['platform.checklists.view', 'platform.checklists.complete', 'platform.checklists.manage']),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY['platform.checklists.view', 'platform.checklists.complete']),
updated_at = NOW()
WHERE name IN ('manager', 'member') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY['platform.checklists.view']),
updated_at = NOW()
WHERE name = 'viewer' AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY['platform.checklists.view', 'platform.checklists.complete', 'platform.checklists.manage'])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN ('platform.checklists.view', 'platform.checklists.complete', 'platform.checklists.manage');

-- +goose StatementEnd
