-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00126_seed_hrm_analytics_permissions
--
-- Phase 10C permissions. One resource, five keys:
--
--   hrm.analytics.view               — headcount, attrition, tenure, cohorts
--   hrm.analytics.view_compensation  — pay distribution
--   hrm.analytics.view_dei           — diversity distribution
--   hrm.analytics.export             — extract rows rather than read a chart
--   hrm.analytics.manage             — author metric definitions, run the job
--
-- ⚠ NOT SCOPE-TIERED. Analytics is aggregate by construction — a headcount
-- trend restricted to your own reports is four dots and a straight line — and
-- the read path touches only fact tables, which have no manager_id to resolve
-- through. No ResolveScope call exists in this package, so
-- TestPermissions_ScopeTiersSeeded does not fire for the resource.
--
-- ⚠ THE PLAN NAMES FOUR KEYS; THIS IS FIVE. `manage` is added deliberately:
-- hrm_metric_definitions is the plan's own non-optional table, and a
-- definition nobody may write is a constant with extra steps. Overloading
-- `export` to mean "may redefine what attrition means" would have been worse
-- than adding the key.
--
-- ⚠ THE THREE READ KEYS ARE SEPARATE BECAUSE THE DATA IS DIFFERENT IN KIND,
-- not because it is more or less sensitive on a single scale.
--
--   • view is ordinary management information: how many people, how many
--     left, how long they stay.
--   • view_compensation exposes what people are paid, in bands. A manager who
--     can see a departmental median can often resolve it to an individual on
--     a small team, which is why the nightly job leaves the columns NULL
--     below the suppression threshold rather than trusting the read path.
--   • view_dei exposes demographic composition. Aggregate only, always.
--
-- ⚠ SUPPRESSION IS NOT A PERMISSION AND NOTHING LIFTS IT. A group below the
-- threshold reports as suppressed to an owner exactly as it does to a
-- manager, at least two groups are suppressed whenever any is, and the total
-- is withheld whenever suppression occurs — because a total minus every
-- disclosed group is the hidden one. view_dei decides whether you see the
-- breakdown at all; it never unlocks a suppressed cell. Same rule as 5C.
--
-- Grant rationale:
--   • owner/admin: all five.
--   • manager: view only. Headcount and attrition trends are management
--     information; pay distribution, demographics, bulk extract and
--     redefining a metric are not.
--   • member/viewer: none. There is no personal analytics view — an
--     employee's own data lives on the records themselves.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.analytics.view',
     'hrm.analytics', 'view',
     'View headcount, attrition, tenure and cohort metrics'),
    ('hrm.analytics.view_compensation',
     'hrm.analytics', 'view_compensation',
     'View aggregate compensation distribution'),
    ('hrm.analytics.view_dei',
     'hrm.analytics', 'view_dei',
     'View aggregate diversity distribution, subject to suppression'),
    ('hrm.analytics.export',
     'hrm.analytics', 'export',
     'Export analytics rows'),
    ('hrm.analytics.manage',
     'hrm.analytics', 'manage',
     'Author metric definitions and run the snapshot job')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.analytics.view',
    'hrm.analytics.view_compensation',
    'hrm.analytics.view_dei',
    'hrm.analytics.export',
    'hrm.analytics.manage'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.analytics.view'
]),
updated_at = NOW()
WHERE name = 'manager' AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY[
        'hrm.analytics.view',
        'hrm.analytics.view_compensation',
        'hrm.analytics.view_dei',
        'hrm.analytics.export',
        'hrm.analytics.manage'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.analytics.view',
    'hrm.analytics.view_compensation',
    'hrm.analytics.view_dei',
    'hrm.analytics.export',
    'hrm.analytics.manage'
);

-- +goose StatementEnd
