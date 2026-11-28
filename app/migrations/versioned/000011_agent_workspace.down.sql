DROP INDEX IF EXISTS idx_sessions_tenant_surface;
DROP INDEX IF EXISTS idx_doc_versions_session;

ALTER TABLE sessions DROP COLUMN IF EXISTS surface;

ALTER TABLE doc_versions
    DROP COLUMN IF EXISTS session_id,
    DROP COLUMN IF EXISTS message_id,
    DROP COLUMN IF EXISTS tool_name;
