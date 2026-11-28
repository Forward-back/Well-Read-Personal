-- SQLite variant of 000011_agent_workspace.

ALTER TABLE doc_versions ADD COLUMN session_id VARCHAR(36);
ALTER TABLE doc_versions ADD COLUMN message_id VARCHAR(36);
ALTER TABLE doc_versions ADD COLUMN tool_name VARCHAR(64);

CREATE INDEX IF NOT EXISTS idx_doc_versions_session
    ON doc_versions (tenant_id, session_id);

ALTER TABLE sessions ADD COLUMN surface VARCHAR(20) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_sessions_tenant_surface
    ON sessions (tenant_id, surface);
