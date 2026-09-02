-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00085_seed_platform_form_permissions
--
-- Phase 5B part 1 permissions, for the form engine primitive.
--
--   platform.forms.view     — read templates and instances
--   platform.forms.respond  — answer and submit a form instance
--   platform.forms.manage   — author templates, sections and questions
--
-- Granted the same way 00077 granted the checklist engine's keys, and for
-- the same reason: an engine's permissions describe capability over the
-- primitive, while each CONSUMER gates its own domain routes with its own
-- keys. An appraisal is reachable through hrm.appraisals.*; this trio only
-- governs the generic form surface.
--
-- .respond reaches 'member' because every consumer has individual
-- contributors filling forms in — an appraisee writing a self-review, a
-- peer giving 360 feedback. As with platform.checklists.complete, the route
-- gate cannot express "is this YOUR form instance", so it does not try: the
-- service narrows to the instance's respondent_user_id.
--
-- viewer gets nothing, and org-created custom roles get nothing until an
-- admin grants explicitly — the 00077/00079/00081/00083 precedent.
-- ============================================================

INSERT INTO permissions (key, resource, action, description) VALUES
    ('platform.forms.view',    'platform.forms', 'view',    'View form templates and instances'),
    ('platform.forms.respond', 'platform.forms', 'respond', 'Answer and submit a form instance assigned to you'),
    ('platform.forms.manage',  'platform.forms', 'manage',  'Author form templates, sections and questions')
ON CONFLICT (key) DO NOTHING;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'platform.forms.view', 'platform.forms.respond', 'platform.forms.manage'
]),
updated_at = NOW()
WHERE name IN ('owner', 'admin') AND org_id IS NULL AND is_system = TRUE;

UPDATE roles
SET permissions = array_cat(permissions, ARRAY[
    'platform.forms.view', 'platform.forms.respond'
]),
updated_at = NOW()
WHERE name IN ('manager', 'member') AND org_id IS NULL AND is_system = TRUE;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

UPDATE roles
SET permissions = ARRAY(
    SELECT unnest(permissions)
    EXCEPT
    SELECT unnest(ARRAY[
        'platform.forms.view', 'platform.forms.respond', 'platform.forms.manage'
    ])
),
updated_at = NOW()
WHERE org_id IS NULL AND is_system = TRUE;

DELETE FROM permissions WHERE key IN (
    'platform.forms.view', 'platform.forms.respond', 'platform.forms.manage'
);

-- +goose StatementEnd
