-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00128_seed_hrm_entities_permissions
--
-- Phase 11A permissions. Two resources, four keys:
--
--   hrm.entities.view    — legal entities and country configs
--   hrm.entities.manage  — create and edit them
--   hrm.locations.view   — work sites
--   hrm.locations.manage — create and edit them
--
-- ⚠ NEITHER IS SCOPE-TIERED. A legal entity and a work site are ORG
-- STRUCTURE, like departments and positions — there is no "your own legal
-- entity" any more than there is your own department. Neither package calls
-- ResolveScope, so TestPermissions_ScopeTiersSeeded does not fire.
--
-- ⚠ ENTITY SCOPING OF DATA IS A SEPARATE PROBLEM AND IS NOT SOLVED HERE.
-- The build plan originally called view_all_entities "a new permission scope
-- tier inside org-level RBAC". It is not becoming one (decided r38): entity
-- membership is orthogonal to the reporting hierarchy that authz.Scope's
-- own/team/all tiers express, and adding a fourth tier would force every
-- scope-tiered resource in HRM to seed a new key or trip
-- TestPermissions_ScopeTiersSeeded's all-or-nothing rule. 11B adds a
-- LegalEntityFilter applied ALONGSIDE the existing tier instead. The keys
-- below govern the entity RECORDS, not what data an entity's members can see.
--
-- ⚠ THESE ARE SEPARATE RESOURCES BECAUSE THEY ANSWER TO DIFFERENT PEOPLE.
-- A legal entity carries a registration number and a tax identifier and is
-- edited by finance or legal; a work site is a building somebody adds when
-- the company opens an office. Folding sites into hrm.entities.manage would
-- mean nobody can add an office without also being able to change the
-- company's tax registration.
--
-- Grant rationale:
--   • owner/admin: all four.
--   • manager/member/viewer: both view keys. Which entity somebody belongs to
--     and which building they work in are ordinary internal facts that appear
--     on an employee record; the registration numbers beside them are the
--     only sensitive part, and reading those is what .view already covers
--     because an entity is not a secret from its own employees.
--   • Nobody but owner/admin gets .manage. Renaming a legal entity or moving
--     a site between entities changes what appears on payslips and statutory
--     filings.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.entities.view',
     'hrm.entities', 'view',
     'View legal entities and country configurations'),
    ('hrm.entities.manage',
     'hrm.entities', 'manage',
     'Create and edit legal entities and country configurations'),
    ('hrm.locations.view',
     'hrm.locations', 'view',
     'View work locations'),
    ('hrm.locations.manage',
     'hrm.locations', 'manage',
     'Create and edit work locations')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.entities.view', 'hrm.entities.manage',
    'hrm.locations.view', 'hrm.locations.manage'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.entities.view', 'hrm.locations.view'
]),
updated_at = NOW()
WHERE name IN ('manager', 'member', 'viewer') AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY[
        'hrm.entities.view', 'hrm.entities.manage',
        'hrm.locations.view', 'hrm.locations.manage'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.entities.view', 'hrm.entities.manage',
    'hrm.locations.view', 'hrm.locations.manage'
);

-- +goose StatementEnd
