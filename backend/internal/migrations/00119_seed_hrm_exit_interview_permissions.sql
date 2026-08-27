-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00119_seed_hrm_exit_interview_permissions
--
-- Phase 9C permissions. Three new actions on the EXISTING hrm.exits
-- resource — no new resource, so hrm.exits' scope tiers (seeded whole in
-- 00115) are untouched and TestPermissions_ScopeTiersSeeded stays satisfied.
--
--   hrm.exits.interview        — schedule, send and cancel exit interviews
--   hrm.exits.interview_view   — READ INDIVIDUAL RESPONSES (confidential)
--   hrm.exits.revoke_access    — run access revocation by hand
--
-- ⚠ .interview_view IS THE SHARPEST KEY IN THIS PHASE, and it is deliberately
-- NOT granted to manager. An exit interview is worth conducting only if the
-- departing employee believes their answers cannot reach the manager they
-- are leaving — and the single most likely reader to want them is exactly
-- that manager. Granting it "to be helpful" destroys the instrument. The
-- scope tiers do not help here either: a manager holds view_team over exits,
-- so confidentiality has to come from a separate action they simply lack.
--
-- .interview (scheduling) is separate from .interview_view (reading) for the
-- same reason 5C split coordination from results: knowing an interview
-- happened is administrative, reading what was said is not.
--
-- .revoke_access exists because the sweep is scheduler-driven but HR
-- sometimes needs to revoke immediately — a dismissal for cause does not
-- wait for tonight's cron. Separate from .manage because it is destructive:
-- it suspends a membership and kills live sessions.
--
-- Grant rationale:
--   • owner/admin: all three.
--   • manager: NONE of them. Not .interview_view (see above), and not
--     .revoke_access — cutting off a colleague's access is an admin act.
--   • member/viewer: nothing.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('hrm.exits.interview',      'hrm.exits', 'interview',      'Schedule, send and cancel exit interviews'),
    ('hrm.exits.interview_view', 'hrm.exits', 'interview_view', 'Read individual exit interview responses (confidential — deliberately withheld from managers)'),
    ('hrm.exits.revoke_access',  'hrm.exits', 'revoke_access',  'Revoke a departing employee''s system access immediately')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'hrm.exits.interview', 'hrm.exits.interview_view', 'hrm.exits.revoke_access'
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
        'hrm.exits.interview', 'hrm.exits.interview_view', 'hrm.exits.revoke_access'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'hrm.exits.interview', 'hrm.exits.interview_view', 'hrm.exits.revoke_access'
);

-- +goose StatementEnd
