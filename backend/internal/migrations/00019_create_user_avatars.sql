-- +goose Up
-- +goose StatementBegin
-- ============================================================
-- Migration: 00020_create_user_avatars
-- Backs the multi-avatar system: up to 3 stored images per user,
-- content-hash deduplicated, at most one marked active at a time.
-- users.photo_url remains the denormalized "currently active" cache
-- so every existing consumer of SafeUser.PhotoURL keeps working
-- unchanged — this table is additive, not a replacement.
-- ============================================================
CREATE TABLE
    user_avatars (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
        user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
        file_path TEXT NOT NULL,
        content_hash CHAR(64) NOT NULL,
        file_size INTEGER NOT NULL,
        width INTEGER NOT NULL,
        height INTEGER NOT NULL,
        is_active BOOLEAN NOT NULL DEFAULT FALSE,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW (),
        UNIQUE (user_id, content_hash)
    );

CREATE INDEX idx_user_avatars_user_id ON user_avatars (user_id);

-- Enforces "at most one active avatar per user" at the database level —
-- application code still coordinates the swap in a transaction, but this
-- index means a bug can never leave a user with two active rows.
CREATE UNIQUE INDEX idx_user_avatars_one_active_per_user ON user_avatars (user_id)
WHERE
    is_active;

COMMENT ON TABLE user_avatars IS 'Up to 3 stored avatar images per user (see MaxAvatarsPerUser in internal/user/avatar.go); exactly one may be is_active.';

COMMENT ON COLUMN user_avatars.file_path IS 'Server-relative path, e.g. /uploads/avatars/<sha256>.webp — resolved to a full URL by the frontend, same convention as users.photo_url.';

COMMENT ON COLUMN user_avatars.content_hash IS 'SHA256 hex digest of the final (cropped+resized+encoded) file bytes. Used both for dedup (UNIQUE with user_id) and as the on-disk filename.';

COMMENT ON COLUMN user_avatars.is_active IS 'Whether this is the users.photo_url currently in effect. Enforced unique-per-user by idx_user_avatars_one_active_per_user.';

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_avatars;

-- +goose StatementEnd