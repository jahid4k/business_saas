-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00045_seed_hrm_group_e_permissions
--
-- Seeds HRM Group E permissions and assigns to system roles.
--
-- Permission matrix:
--   owner / admin  → all Group E permissions
--   manager        → view awards + create/manage awards for team;
--                    view + publish announcements (not org-wide);
--                    manage calendar events; view milestones
--   member         → view own awards, published announcements, calendar events; view own milestones
--   viewer         → read-only across all Group E
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    -- E1: Awards
    ('hrm.awards.view',    'hrm.awards', 'view',    'View employee awards'),
    ('hrm.awards.manage',  'hrm.awards', 'manage',  'Create and update award records'),
    ('hrm.awards.approve', 'hrm.awards', 'approve', 'Approve an award (if approval required)'),
    ('hrm.awards.issue',   'hrm.awards', 'issue',   'Formally issue an award to an employee'),

    -- E2: Announcements
    ('hrm.announcements.view',    'hrm.announcements', 'view',    'View announcements (published)'),
    ('hrm.announcements.manage',  'hrm.announcements', 'manage',  'Create and update announcements'),
    ('hrm.announcements.publish', 'hrm.announcements', 'publish', 'Publish, schedule, or archive announcements'),

    -- E3: HR Calendar
    ('hrm.calendar.view',   'hrm.calendar', 'view',   'View HR calendar events'),
    ('hrm.calendar.manage', 'hrm.calendar', 'manage', 'Create and manage HR calendar events'),

    -- E4: Milestones
    ('hrm.milestones.view',     'hrm.milestones', 'view',     'View employee milestones'),
    ('hrm.milestones.manage',   'hrm.milestones', 'manage',   'Create and update milestones'),
    ('hrm.milestones.generate', 'hrm.milestones', 'generate', 'Bulk-generate upcoming milestones for a period')

ON CONFLICT (key) DO NOTHING;


-- Owner and Admin: full Group E access
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.awards.view',        'hrm.awards.manage',        'hrm.awards.approve',     'hrm.awards.issue',
    'hrm.announcements.view', 'hrm.announcements.manage', 'hrm.announcements.publish',
    'hrm.calendar.view',      'hrm.calendar.manage',
    'hrm.milestones.view',    'hrm.milestones.manage',    'hrm.milestones.generate'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;


-- Manager: create team awards, manage calendar, publish (not org-wide), view milestones
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.awards.view',        'hrm.awards.manage',       'hrm.awards.issue',
    'hrm.announcements.view', 'hrm.announcements.manage','hrm.announcements.publish',
    'hrm.calendar.view',      'hrm.calendar.manage',
    'hrm.milestones.view'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;


-- Member: view own awards + published announcements + calendar + own milestones
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.awards.view',
    'hrm.announcements.view',
    'hrm.calendar.view',
    'hrm.milestones.view'
]),
updated_at = NOW()
WHERE name = 'member' AND org_id IS NULL AND is_system = TRUE;


-- Viewer: read-only
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.awards.view',
    'hrm.announcements.view',
    'hrm.calendar.view',
    'hrm.milestones.view'
]),
updated_at = NOW()
WHERE name = 'viewer' AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY[
        'hrm.awards.view',        'hrm.awards.manage',        'hrm.awards.approve',     'hrm.awards.issue',
        'hrm.announcements.view', 'hrm.announcements.manage', 'hrm.announcements.publish',
        'hrm.calendar.view',      'hrm.calendar.manage',
        'hrm.milestones.view',    'hrm.milestones.manage',    'hrm.milestones.generate'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.awards.view',        'hrm.awards.manage',        'hrm.awards.approve',     'hrm.awards.issue',
    'hrm.announcements.view', 'hrm.announcements.manage', 'hrm.announcements.publish',
    'hrm.calendar.view',      'hrm.calendar.manage',
    'hrm.milestones.view',    'hrm.milestones.manage',    'hrm.milestones.generate'
);

-- +goose StatementEnd
