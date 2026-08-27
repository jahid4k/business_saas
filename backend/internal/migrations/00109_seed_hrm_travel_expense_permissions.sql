-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00109_seed_hrm_travel_expense_permissions
--
-- Phase 8B permissions. Three resources:
--
--   hrm.expense_config — policies + per-diem/mileage rates, NOT scope-tiered
--   hrm.travel         — travel requests, itineraries, advances, SCOPE-TIERED
--   hrm.expenses       — claims + lines, SCOPE-TIERED
--
-- hrm.expense_config covers policies AND both rate tables under one resource
-- — the hrm.rating_scales (00087) / hrm.compensation_config (00099) /
-- hrm.asset_config (00107) precedent. None carries employee_id, so do NOT
-- call ResolveScope for it.
--
-- hrm.travel and hrm.expenses are separate resources rather than one, because
-- the authorities genuinely differ: an org may let a manager approve trips
-- (a scheduling and cost-commitment decision made before the money moves)
-- without letting them settle the money afterward. Merging them would make
-- that ungrantable.
--
-- .approve_lines is its own key and is the sharpest one here. It is the
-- LINE-LEVEL approval the build plan insists on — reducing one line's
-- approved_amount is a money decision distinct from merely viewing or
-- creating a claim, and it is what the whole module's shape exists to
-- support. Deciding the claim's approval INSTANCE still goes through
-- hrm.approvals.action, as everywhere else; .approve_lines is the per-line
-- adjustment that happens before that decision.
--
-- .disburse_advance is separate from .manage for the same reason
-- hrm.loans.disburse was (00101): releasing funds before a trip is a real
-- money movement, distinct from recording that a trip will happen.
--
-- .submit is granted through 'member' — filing your own trip or claim is
-- self-service. The routes cannot express "for yourself only", so the
-- services narrow it by resolving the caller's own employeeID, the
-- hrm.goals.set_own / benefits.EnrollSelf / hrm.assets.request precedent.
--
-- Grant rationale:
--   • owner/admin: everything.
--   • manager: view + view_team on both, plus .approve_lines — a manager
--     reviewing a report's claim is exactly who trims a line. No .manage,
--     no .disburse_advance.
--   • member: view + view_own + .submit on both.
--   • 'viewer' gets nothing, the 00087/.../00107 precedent.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.expense_config.view',   'hrm.expense_config', 'view',   'View expense policies and per-diem/mileage rates'),
    ('hrm.expense_config.manage', 'hrm.expense_config', 'manage', 'Administer expense policies and per-diem/mileage rates'),

    ('hrm.travel.view',              'hrm.travel', 'view',              'View travel requests and advances'),
    ('hrm.travel.manage',            'hrm.travel', 'manage',            'Create and administer travel requests on behalf of others'),
    ('hrm.travel.submit',            'hrm.travel', 'submit',            'File your own travel request'),
    ('hrm.travel.disburse_advance',  'hrm.travel', 'disburse_advance',  'Release a travel advance before the trip'),
    ('hrm.travel.view_own',          'hrm.travel', 'view_own',          'View own travel requests only'),
    ('hrm.travel.view_team',         'hrm.travel', 'view_team',         'View own and direct reports'' travel requests'),
    ('hrm.travel.view_all',          'hrm.travel', 'view_all',          'View all travel requests in the organization'),

    ('hrm.expenses.view',           'hrm.expenses', 'view',           'View expense claims'),
    ('hrm.expenses.manage',         'hrm.expenses', 'manage',         'Administer expense claims on behalf of others'),
    ('hrm.expenses.submit',         'hrm.expenses', 'submit',         'File your own expense claim'),
    ('hrm.expenses.approve_lines',  'hrm.expenses', 'approve_lines',  'Set the approved amount on individual expense lines'),
    ('hrm.expenses.view_own',       'hrm.expenses', 'view_own',       'View own expense claims only'),
    ('hrm.expenses.view_team',      'hrm.expenses', 'view_team',      'View own and direct reports'' expense claims'),
    ('hrm.expenses.view_all',       'hrm.expenses', 'view_all',       'View all expense claims in the organization')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.expense_config.view', 'hrm.expense_config.manage',
    'hrm.travel.view', 'hrm.travel.manage', 'hrm.travel.submit',
    'hrm.travel.disburse_advance', 'hrm.travel.view_all',
    'hrm.expenses.view', 'hrm.expenses.manage', 'hrm.expenses.submit',
    'hrm.expenses.approve_lines', 'hrm.expenses.view_all'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.expense_config.view',
    'hrm.travel.view', 'hrm.travel.submit', 'hrm.travel.view_team',
    'hrm.expenses.view', 'hrm.expenses.submit', 'hrm.expenses.approve_lines', 'hrm.expenses.view_team'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.travel.view', 'hrm.travel.submit', 'hrm.travel.view_own',
    'hrm.expenses.view', 'hrm.expenses.submit', 'hrm.expenses.view_own'
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
        'hrm.expense_config.view', 'hrm.expense_config.manage',
        'hrm.travel.view', 'hrm.travel.manage', 'hrm.travel.submit',
        'hrm.travel.disburse_advance',
        'hrm.travel.view_own', 'hrm.travel.view_team', 'hrm.travel.view_all',
        'hrm.expenses.view', 'hrm.expenses.manage', 'hrm.expenses.submit',
        'hrm.expenses.approve_lines',
        'hrm.expenses.view_own', 'hrm.expenses.view_team', 'hrm.expenses.view_all'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.expense_config.view', 'hrm.expense_config.manage',
    'hrm.travel.view', 'hrm.travel.manage', 'hrm.travel.submit',
    'hrm.travel.disburse_advance',
    'hrm.travel.view_own', 'hrm.travel.view_team', 'hrm.travel.view_all',
    'hrm.expenses.view', 'hrm.expenses.manage', 'hrm.expenses.submit',
    'hrm.expenses.approve_lines',
    'hrm.expenses.view_own', 'hrm.expenses.view_team', 'hrm.expenses.view_all'
);

-- +goose StatementEnd
