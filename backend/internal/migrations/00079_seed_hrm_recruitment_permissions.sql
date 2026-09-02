-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00079_seed_hrm_recruitment_permissions
--
-- Phase 4A permissions. Two resources, not one, because the trust levels
-- genuinely differ:
--
--   hrm.recruitment.*  — requisitions, postings, pipelines, stages. This is
--                        headcount and hiring-process configuration: admin work.
--   hrm.candidates.*   — candidate and application records. This is PII about
--                        people who do not work here yet.
--
-- hrm.candidates.download_resume is separated from .view for the same reason
-- 00075 separated hrm.leave.adjust_balance from hrm.leave.view: the sharpest
-- data in the module deserves its own gate. A hiring manager can be given
-- pipeline visibility without bulk access to every resume on file.
--
-- NOTE ON SCOPING: this module deliberately does NOT use
-- authz.Service.ResolveScope, so no view_own/view_team/view_all tiers are
-- seeded here. internal/hrm/scope's Predicate and AuthorizeRecordAccess both
-- hard-code `FROM hrm_employees`, and candidates and applications are not
-- employees — they have no employee_id for the tiers to resolve against. The
-- consequence is stated plainly rather than papered over: anyone holding
-- hrm.candidates.view sees every candidate in the org. That is acceptable
-- while the grant is owner/admin/manager only. Per-recruiter ownership, if
-- ever wanted, needs an owner_id column and a purpose-built predicate, not
-- the employee-chain tiers.
--
-- member and viewer get nothing. An internal job board for all staff is the
-- public careers page's job, not a permission grant. Org-created custom roles
-- also get nothing until an admin grants them explicitly (the 00077
-- precedent) — these are brand-new capabilities, not a restoration of prior
-- behaviour like 00073's view_own backfill.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.recruitment.view',            'hrm.recruitment', 'view',            'View job requisitions, postings, and hiring pipelines'),
    ('hrm.recruitment.manage',          'hrm.recruitment', 'manage',          'Create and manage requisitions, postings, pipelines, and stages'),
    ('hrm.candidates.view',             'hrm.candidates',  'view',            'View candidate records and applications'),
    ('hrm.candidates.manage',           'hrm.candidates',  'manage',          'Create and manage candidates and move applications through stages'),
    ('hrm.candidates.download_resume',  'hrm.candidates',  'download_resume', 'Download a candidate resume file')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.recruitment.view', 'hrm.recruitment.manage',
    'hrm.candidates.view', 'hrm.candidates.manage', 'hrm.candidates.download_resume'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

-- Managers hire: they need pipeline visibility, candidate management, and
-- resume access — but not requisition/posting configuration, which is where
-- headcount and salary bands are set.
UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.recruitment.view',
    'hrm.candidates.view', 'hrm.candidates.manage', 'hrm.candidates.download_resume'
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
        'hrm.recruitment.view', 'hrm.recruitment.manage',
        'hrm.candidates.view', 'hrm.candidates.manage', 'hrm.candidates.download_resume'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.recruitment.view', 'hrm.recruitment.manage',
    'hrm.candidates.view', 'hrm.candidates.manage', 'hrm.candidates.download_resume'
);

-- +goose StatementEnd
