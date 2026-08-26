-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00103_seed_hrm_statutory_permissions
--
-- Phase 7D (statutory half) permissions. One resource, NOT scope-tiered:
--
--   hrm.statutory — org-level rule/slab configuration, no employee_id
--
-- Same reasoning as hrm.compensation_config (00099) and hrm.rating_scales
-- (00087): a statutory rule/slab is catalog data with no employee_id column,
-- so a scope tier would imply a per-employee filter that cannot exist.
-- Do NOT call ResolveScope for this resource.
--
-- Grant rationale: owner/admin only. Statutory rules decide what is withheld
-- from EVERY employee's pay org-wide — the sharpest-blast-radius
-- configuration surface in Phase 7. No self-service path, no manager
-- visibility grant (unlike hrm.compensation_config, which manager can view) —
-- getting a tax bracket wrong is a compliance incident, not a judgment call
-- a manager should even be able to see the mechanics of.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.statutory.view',   'hrm.statutory', 'view',   'View statutory rules and slabs'),
    ('hrm.statutory.manage', 'hrm.statutory', 'manage', 'Administer statutory rules and slabs')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY['hrm.statutory.view', 'hrm.statutory.manage']),
    updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions) EXCEPT SELECT unnest(ARRAY['hrm.statutory.view', 'hrm.statutory.manage'])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN ('hrm.statutory.view', 'hrm.statutory.manage');

-- +goose StatementEnd
