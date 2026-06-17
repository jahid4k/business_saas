-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00016_update_roles_crm_permissions
-- Adds CRM and platform contact/company permissions to system roles.
-- Depends on: 00014_seed_platform_crm_permissions (permission rows exist)
--             00004_create_roles (role rows exist)
-- ============================================================

-- Owner and Admin: all CRM permissions
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'crm.contacts.view',   'crm.contacts.create',   'crm.contacts.update',   'crm.contacts.delete',
    'crm.companies.view',  'crm.companies.create',  'crm.companies.update',  'crm.companies.delete',
    'crm.leads.view',      'crm.leads.create',      'crm.leads.update',      'crm.leads.delete',    'crm.leads.convert',
    'crm.deals.view',      'crm.deals.create',      'crm.deals.update',      'crm.deals.delete',    'crm.deals.move_stage',
    'crm.tasks.view',      'crm.tasks.create',       'crm.tasks.update',      'crm.tasks.delete',    'crm.tasks.assign',
    'crm.activities.view', 'crm.activities.create', 'crm.activities.update', 'crm.activities.delete',
    'crm.notes.view',      'crm.notes.create',      'crm.notes.update',      'crm.notes.delete',
    'crm.emails.view',     'crm.emails.create',     'crm.emails.delete',
    'crm.reports.view'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

-- Manager: create/update/view for all CRM, no delete, can convert leads and move deals
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'crm.contacts.view',   'crm.contacts.create',   'crm.contacts.update',
    'crm.companies.view',  'crm.companies.create',  'crm.companies.update',
    'crm.leads.view',      'crm.leads.create',      'crm.leads.update',      'crm.leads.convert',
    'crm.deals.view',      'crm.deals.create',      'crm.deals.update',      'crm.deals.move_stage',
    'crm.tasks.view',      'crm.tasks.create',       'crm.tasks.update',      'crm.tasks.assign',
    'crm.activities.view', 'crm.activities.create', 'crm.activities.update',
    'crm.notes.view',      'crm.notes.create',      'crm.notes.update',
    'crm.emails.view',     'crm.emails.create',
    'crm.reports.view'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

-- Member: view + create + update, no delete, no reports
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'crm.contacts.view',   'crm.contacts.create',   'crm.contacts.update',
    'crm.companies.view',  'crm.companies.create',
    'crm.leads.view',      'crm.leads.create',      'crm.leads.update',
    'crm.deals.view',      'crm.deals.create',       'crm.deals.update',     'crm.deals.move_stage',
    'crm.tasks.view',      'crm.tasks.create',       'crm.tasks.update',
    'crm.activities.view', 'crm.activities.create',
    'crm.notes.view',      'crm.notes.create',
    'crm.emails.view'
]),
updated_at = NOW()
WHERE name = 'member' AND org_id IS NULL AND is_system = TRUE;

-- Viewer: read-only
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'crm.contacts.view',
    'crm.companies.view',
    'crm.leads.view',
    'crm.deals.view',
    'crm.tasks.view',
    'crm.activities.view',
    'crm.notes.view',
    'crm.emails.view',
    'crm.reports.view'
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
        'crm.contacts.view',   'crm.contacts.create',   'crm.contacts.update',   'crm.contacts.delete',
        'crm.companies.view',  'crm.companies.create',  'crm.companies.update',  'crm.companies.delete',
        'crm.leads.view',      'crm.leads.create',      'crm.leads.update',      'crm.leads.delete',    'crm.leads.convert',
        'crm.deals.view',      'crm.deals.create',      'crm.deals.update',      'crm.deals.delete',    'crm.deals.move_stage',
        'crm.tasks.view',      'crm.tasks.create',       'crm.tasks.update',      'crm.tasks.delete',    'crm.tasks.assign',
        'crm.activities.view', 'crm.activities.create', 'crm.activities.update', 'crm.activities.delete',
        'crm.notes.view',      'crm.notes.create',      'crm.notes.update',      'crm.notes.delete',
        'crm.emails.view',     'crm.emails.create',     'crm.emails.delete',
        'crm.reports.view'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd