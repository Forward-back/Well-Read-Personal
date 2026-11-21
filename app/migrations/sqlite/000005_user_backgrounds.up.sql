-- Personal shell background: metadata only; image bytes live in the app
-- data volume. See migrations/versioned/000005 for the Postgres variant.

ALTER TABLE users ADD COLUMN background_choice TEXT NOT NULL DEFAULT 'default';

CREATE TABLE IF NOT EXISTS user_backgrounds (
    id           VARCHAR(36) PRIMARY KEY,
    user_id      VARCHAR(36) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    mime_type    VARCHAR(100) NOT NULL,
    size_bytes   BIGINT NOT NULL,
    file_path    TEXT NOT NULL,
    created_at   DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_user_backgrounds_user_id ON user_backgrounds (user_id);
