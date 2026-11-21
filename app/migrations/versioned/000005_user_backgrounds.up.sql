-- Personal shell background: the owner can pick a built-in wallpaper or
-- upload their own images. Uploaded files live in the app data volume; the
-- table keeps metadata only. background_choice stores "default",
-- "builtin:N" or "upload:<id>".

ALTER TABLE users ADD COLUMN IF NOT EXISTS background_choice VARCHAR(64) NOT NULL DEFAULT 'default';

CREATE TABLE IF NOT EXISTS user_backgrounds (
    id           VARCHAR(36) PRIMARY KEY,
    user_id      VARCHAR(36) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    mime_type    VARCHAR(100) NOT NULL,
    size_bytes   BIGINT NOT NULL,
    file_path    TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_user_backgrounds_user_id ON user_backgrounds (user_id);
