-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00037_seed_hrm_group_c_permissions
--
-- Seeds HRM Group C permissions and assigns to system roles.
--
-- Permission matrix:
--   owner / admin  → all Group C permissions
--   manager        → view all + issue warnings + manage complaints (not close/resolve) + view docs
--   member         → view own warnings, submit complaints, acknowledge, view own docs
--   viewer         → view warnings/complaints (read-only, no employee data)
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    -- C1: Employee Warnings
    ('hrm.warnings.view',        'hrm.warnings',        'view',        'View employee warning records'),
    ('hrm.warnings.manage',      'hrm.warnings',        'manage',      'Create and update warning records'),
    ('hrm.warnings.issue',       'hrm.warnings',        'issue',       'Formally issue a warning to an employee'),
    ('hrm.warnings.acknowledge', 'hrm.warnings',        'acknowledge', 'Acknowledge or appeal a warning (employee action)'),
    ('hrm.warnings.close',       'hrm.warnings',        'close',       'Close or cancel a warning (HR action)'),

    -- C2: Complaints
    ('hrm.complaints.view',      'hrm.complaints',      'view',        'View complaint records'),
    ('hrm.complaints.manage',    'hrm.complaints',      'manage',      'Submit and update complaints'),
    ('hrm.complaints.process',   'hrm.complaints',      'process',     'Investigate and resolve complaints (HR action)'),

    -- C3: Employee Documents
    ('hrm.documents.view',       'hrm.documents',       'view',        'View employee document records'),
    ('hrm.documents.manage',     'hrm.documents',       'manage',      'Create, send, and manage employee documents'),
    ('hrm.documents.acknowledge','hrm.documents',       'acknowledge', 'Acknowledge or decline a received document (employee)'),

    -- C4: Acknowledgements
    ('hrm.acknowledgements.view',    'hrm.acknowledgements', 'view',    'View acknowledgement records'),
    ('hrm.acknowledgements.manage',  'hrm.acknowledgements', 'manage',  'Create and manage acknowledgement requests (HR)'),
    ('hrm.acknowledgements.respond', 'hrm.acknowledgements', 'respond', 'Acknowledge or decline an acknowledgement request (employee)')

ON CONFLICT (key) DO NOTHING;


-- Owner and Admin: full Group C access
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.warnings.view',         'hrm.warnings.manage',      'hrm.warnings.issue',
    'hrm.warnings.acknowledge',  'hrm.warnings.close',
    'hrm.complaints.view',       'hrm.complaints.manage',    'hrm.complaints.process',
    'hrm.documents.view',        'hrm.documents.manage',     'hrm.documents.acknowledge',
    'hrm.acknowledgements.view', 'hrm.acknowledgements.manage', 'hrm.acknowledgements.respond'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;


-- Manager: can view and issue warnings, start complaint review, view docs
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.warnings.view',   'hrm.warnings.manage',   'hrm.warnings.issue',
    'hrm.complaints.view', 'hrm.complaints.manage',  'hrm.complaints.process',
    'hrm.documents.view',
    'hrm.acknowledgements.view', 'hrm.acknowledgements.manage', 'hrm.acknowledgements.respond'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;


-- Member: employee self-service — view own warnings, submit complaints, acknowledge
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.warnings.view',        'hrm.warnings.acknowledge',
    'hrm.complaints.manage',
    'hrm.documents.view',       'hrm.documents.acknowledge',
    'hrm.acknowledgements.respond'
]),
updated_at = NOW()
WHERE name = 'member' AND org_id IS NULL AND is_system = TRUE;


-- Viewer: read-only
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.warnings.view',
    'hrm.complaints.view',
    'hrm.documents.view',
    'hrm.acknowledgements.view'
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
        'hrm.warnings.view',         'hrm.warnings.manage',      'hrm.warnings.issue',
        'hrm.warnings.acknowledge',  'hrm.warnings.close',
        'hrm.complaints.view',       'hrm.complaints.manage',    'hrm.complaints.process',
        'hrm.documents.view',        'hrm.documents.manage',     'hrm.documents.acknowledge',
        'hrm.acknowledgements.view', 'hrm.acknowledgements.manage', 'hrm.acknowledgements.respond'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.warnings.view',         'hrm.warnings.manage',      'hrm.warnings.issue',
    'hrm.warnings.acknowledge',  'hrm.warnings.close',
    'hrm.complaints.view',       'hrm.complaints.manage',    'hrm.complaints.process',
    'hrm.documents.view',        'hrm.documents.manage',     'hrm.documents.acknowledge',
    'hrm.acknowledgements.view', 'hrm.acknowledgements.manage', 'hrm.acknowledgements.respond'
);

-- +goose StatementEnd
