-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00113_seed_platform_kb_permissions
--
-- Phase 8D permissions. One resource, platform-namespaced to match
-- platform.checklists (00077), platform.forms (00085) and
-- platform.tickets (00111):
--
--   platform.kb — knowledge base categories and articles
--
-- ⚠ NOT scope-tiered, for the same structural reason as platform.tickets:
-- internal/hrm/scope's Predicate hard-codes FROM hrm_employees, and no
-- platform package may depend on that. There is no ResolveScope call in
-- internal/platform/kb, so TestPermissions_ScopeTiersSeeded does not fire.
--
-- TWO KEYS ONLY, and the draft/published split is what makes that enough.
-- A knowledge base is org-wide reading material — there is no "my articles"
-- to narrow to, unlike tickets. The one thing that DOES need protecting is
-- unpublished work: a half-written HR policy read as authoritative is worse
-- than no article at all. So .view sees published articles and .manage sees
-- everything, enforced in the service rather than by a third key. A separate
-- .view_unpublished would imply a contributor role this product does not
-- have yet, and an unused key is one nobody notices is granted wrongly.
--
-- Grant rationale:
--   • owner/admin: .view + .manage.
--   • manager: .view + .manage — helpdesk agents are exactly who writes and
--     corrects KB articles, and they already hold the agent ticket keys.
--   • member: .view. The KB exists to be read by employees; that is the
--     whole point of pairing it with the helpdesk.
--   • viewer: .view. Unlike tickets — where a read-only role has no business
--     reading other people's helpdesk threads — published documentation is
--     precisely what a read-only role should see.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('platform.kb.view',   'platform.kb', 'view',   'Read published knowledge base articles'),
    ('platform.kb.manage', 'platform.kb', 'manage', 'Write, publish and archive knowledge base articles, including drafts')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'platform.kb.view', 'platform.kb.manage'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin', 'manager') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'platform.kb.view'
]),
updated_at = NOW()
WHERE name IN ('member', 'viewer') AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY[
        'platform.kb.view', 'platform.kb.manage'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'platform.kb.view', 'platform.kb.manage'
);

-- +goose StatementEnd
