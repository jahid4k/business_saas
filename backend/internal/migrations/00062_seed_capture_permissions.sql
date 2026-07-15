-- +goose Up
-- +goose StatementBegin

INSERT INTO permissions (key, resource, action, description)
VALUES
    ('capture.apikeys.view',   'capture.apikeys', 'view',   'View API Keys'),
    ('capture.apikeys.create', 'capture.apikeys', 'create', 'Create API Keys'),
    ('capture.apikeys.delete', 'capture.apikeys', 'delete', 'Delete API Keys'),
    ('capture.email.manage',   'capture.email',   'manage', 'Manage Email Capture Settings'),
    ('capture.social.manage',  'capture.social',  'manage', 'Manage Social Integrations'),
    ('capture.visitors.view',  'capture.visitors', 'view',  'View Website Visitors')
ON CONFLICT (key) DO NOTHING;

-- Owner and Admin: all capture permissions
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'capture.apikeys.view', 'capture.apikeys.create', 'capture.apikeys.delete',
    'capture.email.manage', 'capture.social.manage', 'capture.visitors.view'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY[
        'capture.apikeys.view', 'capture.apikeys.create', 'capture.apikeys.delete',
        'capture.email.manage', 'capture.social.manage', 'capture.visitors.view'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd
