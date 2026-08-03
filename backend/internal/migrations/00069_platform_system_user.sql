-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Migration: 00069_platform_system_user
-- Seeds a sentinel "system" user row so scheduler-triggered jobs
-- have a valid actor to satisfy NOT NULL created_by FKs (e.g.
-- hrm_employee_milestones.created_by, hrm_attendance_records.created_by).
-- Never a real login: email/password_hash are left NULL.
-- ============================================================

INSERT INTO users (id, email, password_hash, display_name, first_name, last_name, full_name, email_verified, timezone, locale, language)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    NULL, NULL,
    'System', 'System', '', 'System',
    TRUE, 'UTC', 'en', 'en'
)
ON CONFLICT (id) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM users WHERE id = '00000000-0000-0000-0000-000000000001';

-- +goose StatementEnd
