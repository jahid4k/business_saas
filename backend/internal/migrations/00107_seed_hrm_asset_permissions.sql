-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00107_seed_hrm_asset_permissions
--
-- Phase 8A permissions. Two resources:
--
--   hrm.asset_config — categories + software licences, NOT scope-tiered
--   hrm.assets       — instances, assignments, maintenance, requests, SCOPE-TIERED
--
-- hrm.asset_config covers BOTH hrm_asset_categories and hrm_software_licenses
-- under one resource — the hrm.rating_scales (00087) / hrm.compensation_config
-- (00099) / hrm.benefit_plans (00105) precedent. Neither table carries an
-- employee_id, so do NOT call ResolveScope for this resource; a tier there
-- would imply a per-employee filter that cannot exist on catalog data.
--
-- hrm.assets IS scope-tiered. The tiers govern ASSIGNMENTS and REQUESTS, which
-- carry employee_id; hrm_assets itself does not. Note the asymmetry, because
-- it reads wrong at a glance and has the same shape as hrm.payroll's (00097):
-- a member with view_own sees the assets assigned to THEM, resolved through
-- hrm_asset_assignments.employee_id, not a filter on hrm_assets.
--
-- .request is its own key, granted through 'member', because requesting an
-- asset is genuinely self-service — the hrm.goals.set_own / 
-- hrm.benefit_enrollments.enroll_self precedent. The route cannot express
-- "for yourself only", so the service narrows it.
--
-- .assign is distinct from .manage: handing an asset to a person and editing
-- the asset catalog are different authorities, and an org may well let IT
-- assign hardware without letting them create or retire asset records. Same
-- reasoning that split hrm.loans.disburse from hrm.loans.manage (00101).
--
-- Deciding an asset request's approval instance goes through
-- hrm.approvals.action, not a permission here — the 00099/00101 precedent.
--
-- Grant rationale:
--   • owner/admin get everything, including .manage and .assign.
--   • manager gets view + view_team + .request — visibility into a report's
--     equipment without the authority to assign or edit the catalog.
--   • member gets view + view_own + .request.
--   • 'viewer' gets nothing, the 00087/.../00105 precedent.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.asset_config.view',   'hrm.asset_config', 'view',   'View asset categories and software licences'),
    ('hrm.asset_config.manage', 'hrm.asset_config', 'manage', 'Administer asset categories and software licences'),

    ('hrm.assets.view',      'hrm.assets', 'view',      'View assets, assignments and maintenance history'),
    ('hrm.assets.manage',    'hrm.assets', 'manage',    'Create, edit and retire asset records'),
    ('hrm.assets.assign',    'hrm.assets', 'assign',    'Assign an asset to an employee and record its return'),
    ('hrm.assets.request',   'hrm.assets', 'request',   'Raise an asset request for yourself'),
    ('hrm.assets.view_own',  'hrm.assets', 'view_own',  'View own asset assignments only'),
    ('hrm.assets.view_team', 'hrm.assets', 'view_team', 'View own and direct reports'' asset assignments'),
    ('hrm.assets.view_all',  'hrm.assets', 'view_all',  'View all asset assignments in the organization')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.asset_config.view', 'hrm.asset_config.manage',
    'hrm.assets.view', 'hrm.assets.manage', 'hrm.assets.assign',
    'hrm.assets.request', 'hrm.assets.view_all'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.asset_config.view',
    'hrm.assets.view', 'hrm.assets.request', 'hrm.assets.view_team'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.assets.view', 'hrm.assets.request', 'hrm.assets.view_own'
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
        'hrm.asset_config.view', 'hrm.asset_config.manage',
        'hrm.assets.view', 'hrm.assets.manage', 'hrm.assets.assign', 'hrm.assets.request',
        'hrm.assets.view_own', 'hrm.assets.view_team', 'hrm.assets.view_all'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.asset_config.view', 'hrm.asset_config.manage',
    'hrm.assets.view', 'hrm.assets.manage', 'hrm.assets.assign', 'hrm.assets.request',
    'hrm.assets.view_own', 'hrm.assets.view_team', 'hrm.assets.view_all'
);

-- +goose StatementEnd
