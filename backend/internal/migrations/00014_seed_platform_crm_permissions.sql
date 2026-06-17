-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00014_seed_platform_crm_permissions
-- Seeds platform engagement permissions and CRM module permissions.
-- Platform permissions are shared across all future modules.
-- CRM permissions are CRM-specific.
--
-- NOTE: This migration seeds into the `permissions` table created in
-- 00003_create_permissions. The platform_ and crm_ tables that these
-- permissions guard are created in 00015_create_platform_and_crm_tables.
-- The permission rows themselves do not depend on those tables.
-- ============================================================

-- Platform engagement permissions (shared: crm, erp, hrm all write here)
INSERT INTO permissions (key, resource, action, description) VALUES
    ('platform.contacts.view',   'platform.contacts',   'view',   'View platform contacts'),
    ('platform.contacts.create', 'platform.contacts',   'create', 'Create platform contacts'),
    ('platform.contacts.update', 'platform.contacts',   'update', 'Update platform contacts'),
    ('platform.contacts.delete', 'platform.contacts',   'delete', 'Delete platform contacts'),
    ('platform.companies.view',  'platform.companies',  'view',   'View platform companies'),
    ('platform.companies.create','platform.companies',  'create', 'Create platform companies'),
    ('platform.companies.update','platform.companies',  'update', 'Update platform companies'),
    ('platform.companies.delete','platform.companies',  'delete', 'Delete platform companies')
ON CONFLICT (key) DO NOTHING;

-- CRM-specific permissions.
-- contacts and companies are accessed through platform.* permissions globally,
-- but crm.contacts.* aliases exist so CRM routes can use a consistent prefix.
-- When HRM arrives it will use hrm.employees.* for its own employee records,
-- but platform.contacts.* for shared contact data.
INSERT INTO permissions (key, resource, action, description) VALUES
    ('crm.leads.view',       'crm.leads',      'view',       'View CRM leads'),
    ('crm.leads.create',     'crm.leads',      'create',     'Create CRM leads'),
    ('crm.leads.update',     'crm.leads',      'update',     'Update CRM leads'),
    ('crm.leads.delete',     'crm.leads',      'delete',     'Delete CRM leads'),
    ('crm.leads.convert',    'crm.leads',      'convert',    'Convert leads to contacts/deals'),
    ('crm.deals.view',       'crm.deals',      'view',       'View CRM deals'),
    ('crm.deals.create',     'crm.deals',      'create',     'Create CRM deals'),
    ('crm.deals.update',     'crm.deals',      'update',     'Update CRM deals'),
    ('crm.deals.delete',     'crm.deals',      'delete',     'Delete CRM deals'),
    ('crm.deals.move_stage', 'crm.deals',      'move_stage', 'Move deals between pipeline stages'),
    ('crm.tasks.view',       'crm.tasks',      'view',       'View CRM tasks'),
    ('crm.tasks.create',     'crm.tasks',      'create',     'Create CRM tasks'),
    ('crm.tasks.update',     'crm.tasks',      'update',     'Update CRM tasks'),
    ('crm.tasks.delete',     'crm.tasks',      'delete',     'Delete CRM tasks'),
    ('crm.tasks.assign',     'crm.tasks',      'assign',     'Assign CRM tasks to members'),
    ('crm.activities.view',  'crm.activities', 'view',       'View CRM activities'),
    ('crm.activities.create','crm.activities', 'create',     'Create CRM activities'),
    ('crm.activities.update','crm.activities', 'update',     'Update CRM activities'),
    ('crm.activities.delete','crm.activities', 'delete',     'Delete CRM activities'),
    ('crm.notes.view',       'crm.notes',      'view',       'View CRM notes'),
    ('crm.notes.create',     'crm.notes',      'create',     'Create CRM notes'),
    ('crm.notes.update',     'crm.notes',      'update',     'Update CRM notes'),
    ('crm.notes.delete',     'crm.notes',      'delete',     'Delete CRM notes'),
    ('crm.emails.view',      'crm.emails',     'view',       'View CRM email logs'),
    ('crm.emails.create',    'crm.emails',     'create',     'Create CRM email logs'),
    ('crm.emails.delete',    'crm.emails',     'delete',     'Delete CRM email logs'),
    ('crm.reports.view',     'crm.reports',    'view',       'View CRM reports and dashboards'),
    ('crm.contacts.view',    'crm.contacts',   'view',       'View CRM contacts (via platform)'),
    ('crm.contacts.create',  'crm.contacts',   'create',     'Create CRM contacts (via platform)'),
    ('crm.contacts.update',  'crm.contacts',   'update',     'Update CRM contacts (via platform)'),
    ('crm.contacts.delete',  'crm.contacts',   'delete',     'Delete CRM contacts (via platform)'),
    ('crm.companies.view',   'crm.companies',  'view',       'View CRM companies (via platform)'),
    ('crm.companies.create', 'crm.companies',  'create',     'Create CRM companies (via platform)'),
    ('crm.companies.update', 'crm.companies',  'update',     'Update CRM companies (via platform)'),
    ('crm.companies.delete', 'crm.companies',  'delete',     'Delete CRM companies (via platform)')
ON CONFLICT (key) DO NOTHING;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

DELETE FROM permissions WHERE key LIKE 'crm.%';
DELETE FROM permissions WHERE key LIKE 'platform.contacts.%';
DELETE FROM permissions WHERE key LIKE 'platform.companies.%';

-- +goose StatementEnd