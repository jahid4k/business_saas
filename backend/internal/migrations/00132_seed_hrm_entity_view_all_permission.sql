-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00132_seed_hrm_entity_view_all_permission
--
-- Phase 11B-2 permissions. ONE key:
--
--   hrm.entities.view_all — see records belonging to every legal entity
--
-- ⚠ THIS IS AN ORDINARY PERMISSION, NOT A FOURTH authz.Scope TIER, and that
-- decision (r38) is the reason this migration is four lines of INSERT rather
-- than a rewrite of every scope-tiered resource in HRM.
--
-- authz.Scope expresses own/team/all, and those tiers resolve through the
-- REPORTING HIERARCHY (hrm_employees.manager_id, via scope.Predicate's
-- recursive CTE). Entity membership is orthogonal to that: your manager and
-- your employing company are unrelated facts. Adding a fourth tier would
-- have forced every one of the scope-tiered HRM resources to seed a new key
-- or trip TestPermissions_ScopeTiersSeeded's all-or-nothing rule, to express
-- something the tier vocabulary does not mean.
--
-- So entity scoping is a SEPARATE DIMENSION applied ALONGSIDE the tier:
--
--     (your own/team/all tier) AND (your entity, unless you hold view_all)
--
-- ⚠ WITHOUT view_all, A CALLER IS NOT RESTRICTED TO NOTHING — they are
-- restricted to their OWN entity, and an organization with no entities has
-- no restriction at all. That is what keeps every existing org unaffected:
-- entity filtering that has no entities to filter by is a no-op, not a
-- lockout. A filter that failed closed here would empty the employee list of
-- every organization in this database on the day it shipped.
--
-- Grant rationale:
--   • owner/admin: granted. Running payroll for the whole group, and seeing
--     group-wide analytics, is exactly their job.
--   • manager/member/viewer: NOT granted. A manager in the UK subsidiary
--     sees the UK subsidiary. This is the first key in the product that is
--     about WHICH COMPANY somebody belongs to rather than where they sit in
--     a reporting line.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.entities.view_all',
     'hrm.entities', 'view_all',
     'See records belonging to every legal entity, not only your own')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY['hrm.entities.view_all']),
    updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions) EXCEPT SELECT unnest(ARRAY['hrm.entities.view_all'])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key = 'hrm.entities.view_all';

-- +goose StatementEnd
