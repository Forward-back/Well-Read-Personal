-- Agent workspace: mark workspace sessions apart from chat/reading sessions and
-- record which agent turn produced each document version.

ALTER TABLE doc_versions ADD COLUMN IF NOT EXISTS session_id VARCHAR(36);
ALTER TABLE doc_versions ADD COLUMN IF NOT EXISTS message_id VARCHAR(36);
ALTER TABLE doc_versions ADD COLUMN IF NOT EXISTS tool_name VARCHAR(64);

CREATE INDEX IF NOT EXISTS idx_doc_versions_session
    ON doc_versions (tenant_id, session_id);

ALTER TABLE sessions ADD COLUMN IF NOT EXISTS surface VARCHAR(20) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_sessions_tenant_surface
    ON sessions (tenant_id, surface);
