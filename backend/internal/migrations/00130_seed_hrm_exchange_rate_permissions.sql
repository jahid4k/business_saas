-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00130_seed_hrm_exchange_rate_permissions
--
-- Phase 11B-1 permissions. One resource, two keys:
--
--   hrm.exchange_rates.view   — read the rate table
--   hrm.exchange_rates.manage — record rates
--
-- ⚠ NOT SCOPE-TIERED. An exchange rate is a fact about two currencies on a
-- date; there is no "your own USD→EUR rate". No ResolveScope call exists in
-- this package, so TestPermissions_ScopeTiersSeeded does not fire.
--
-- ⚠ A SEPARATE RESOURCE FROM hrm.entities, for the same reason hrm.locations
-- was split from it in 11A: the two are edited by different people on
-- different rhythms. Rates are entered weekly or daily by finance; a legal
-- entity's tax registration is edited once a year by legal. Folding rates
-- into hrm.entities.manage would mean nobody can record Monday's rate
-- without also being able to change the company's registered address.
--
-- ⚠ view IS GRANTED WIDELY AND THAT IS DELIBERATE. A rate is the thing that
-- explains why somebody's foreign expense claim converted to the figure it
-- did. An employee who can see the claim and not the rate behind it has been
-- handed a number they cannot check, which is the whole failure mode the
-- never-store-converted-only rule exists to prevent. Rates are not
-- confidential; they are published facts about currencies.
--
-- Grant rationale:
--   • owner/admin: view + manage.
--   • manager/member/viewer: view. Reading the rate that priced your own
--     claim is not a privilege.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.exchange_rates.view',
     'hrm.exchange_rates', 'view',
     'View recorded currency exchange rates'),
    ('hrm.exchange_rates.manage',
     'hrm.exchange_rates', 'manage',
     'Record currency exchange rates')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.exchange_rates.view', 'hrm.exchange_rates.manage'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.exchange_rates.view'
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
    SELECT unnest(ARRAY['hrm.exchange_rates.view', 'hrm.exchange_rates.manage'])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.exchange_rates.view', 'hrm.exchange_rates.manage'
);

-- +goose StatementEnd
