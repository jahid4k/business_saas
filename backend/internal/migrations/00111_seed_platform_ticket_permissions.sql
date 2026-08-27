-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00111_seed_platform_ticket_permissions
--
-- Phase 8C permissions. Two resources, both PLATFORM-namespaced to match
-- platform.checklists (00077) and platform.forms (00085):
--
--   platform.ticket_config — categories + SLA policies, admin-only
--   platform.tickets       — the tickets themselves
--
-- ⚠ NEITHER IS SCOPE-TIERED, and that is a consequence of the platform
-- decision, not an oversight. internal/hrm/scope's Predicate hard-codes
-- FROM hrm_employees, so a platform package cannot use it without gaining
-- exactly the hrm dependency this fork was resolved to avoid. There is
-- therefore no ResolveScope call anywhere in internal/platform/tickets, and
-- TestPermissions_ScopeTiersSeeded does not fire for these resources.
--
-- "See only my own tickets" is instead narrowed IN THE SERVICE, against
-- requester_user_id and assignee_user_id — the platform.checklists.complete
-- precedent, whose own description reads "…or view items assigned to the
-- caller". A member holding platform.tickets.view sees the tickets they
-- raised; an agent additionally sees the ones assigned to them; only
-- .view_all lifts that to the whole org.
--
-- .comment_internal is the sharpest key here. Internal comments are
-- agent-to-agent and the requester must never see them — the filtering is
-- structural at the repository layer, but WRITING one still needs its own
-- authority, or any requester could plant a comment their own read path
-- then hides from them.
--
-- .assign is separate from .manage for the sensitive-category reason: a
-- category may restrict its assignee pool to one role, and assignment is
-- where that rule is enforced.
--
-- Grant rationale:
--   • owner/admin: everything, including .view_all and ticket_config.
--   • manager: the agent role — view, comment, .comment_internal, .assign,
--     .resolve, .pause. NOT .view_all: a manager works their queue, and
--     org-wide visibility over every helpdesk ticket (which may include
--     sensitive categories) is a deliberate admin capability.
--   • member: raise a ticket, view their own, comment publicly. No
--     .comment_internal, no .assign.
--   • viewer: nothing. Unlike platform.checklists, a read-only role has no
--     business reading other people's helpdesk tickets.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('platform.ticket_config.view',   'platform.ticket_config', 'view',   'View ticket categories and SLA policies'),
    ('platform.ticket_config.manage', 'platform.ticket_config', 'manage', 'Administer ticket categories and SLA policies'),

    ('platform.tickets.view',             'platform.tickets', 'view',             'View tickets you raised or are assigned'),
    ('platform.tickets.create',           'platform.tickets', 'create',           'Raise a ticket'),
    ('platform.tickets.comment',          'platform.tickets', 'comment',          'Add a public comment to a ticket'),
    ('platform.tickets.comment_internal', 'platform.tickets', 'comment_internal', 'Add an internal-only comment the requester never sees'),
    ('platform.tickets.assign',           'platform.tickets', 'assign',           'Assign a ticket, honouring sensitive-category restrictions'),
    ('platform.tickets.resolve',          'platform.tickets', 'resolve',          'Resolve, close, or reopen a ticket'),
    ('platform.tickets.pause',            'platform.tickets', 'pause',            'Pause and resume a ticket''s SLA clock'),
    ('platform.tickets.view_all',         'platform.tickets', 'view_all',         'View every ticket in the organization')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'platform.ticket_config.view', 'platform.ticket_config.manage',
    'platform.tickets.view', 'platform.tickets.create', 'platform.tickets.comment',
    'platform.tickets.comment_internal', 'platform.tickets.assign',
    'platform.tickets.resolve', 'platform.tickets.pause', 'platform.tickets.view_all'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'platform.ticket_config.view',
    'platform.tickets.view', 'platform.tickets.create', 'platform.tickets.comment',
    'platform.tickets.comment_internal', 'platform.tickets.assign',
    'platform.tickets.resolve', 'platform.tickets.pause'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'platform.tickets.view', 'platform.tickets.create', 'platform.tickets.comment'
]),
updated_at = NOW()
WHERE name = 'member' AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY[
        'platform.ticket_config.view', 'platform.ticket_config.manage',
        'platform.tickets.view', 'platform.tickets.create', 'platform.tickets.comment',
        'platform.tickets.comment_internal', 'platform.tickets.assign',
        'platform.tickets.resolve', 'platform.tickets.pause', 'platform.tickets.view_all'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'platform.ticket_config.view', 'platform.ticket_config.manage',
    'platform.tickets.view', 'platform.tickets.create', 'platform.tickets.comment',
    'platform.tickets.comment_internal', 'platform.tickets.assign',
    'platform.tickets.resolve', 'platform.tickets.pause', 'platform.tickets.view_all'
);

-- +goose StatementEnd
