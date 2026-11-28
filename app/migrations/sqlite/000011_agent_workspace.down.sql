DROP INDEX IF EXISTS idx_sessions_tenant_surface;
DROP INDEX IF EXISTS idx_doc_versions_session;

ALTER TABLE sessions DROP COLUMN surface;

ALTER TABLE doc_versions DROP COLUMN session_id;
ALTER TABLE doc_versions DROP COLUMN message_id;
ALTER TABLE doc_versions DROP COLUMN tool_name;
