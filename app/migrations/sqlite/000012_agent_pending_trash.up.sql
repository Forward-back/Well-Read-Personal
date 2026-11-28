-- SQLite variant of 000012_agent_pending_trash.

CREATE TABLE IF NOT EXISTS agent_pending_tasks (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    user_id VARCHAR(512) NOT NULL DEFAULT '',
    session_id VARCHAR(36) NOT NULL DEFAULT '',
    message_id VARCHAR(36) NOT NULL DEFAULT '',
    tool_call_id VARCHAR(36) NOT NULL DEFAULT '',
    tool_name VARCHAR(64) NOT NULL,
    risk_level VARCHAR(8) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    target_knowledge_ids TEXT NOT NULL DEFAULT '[]',
    dry_run TEXT NOT NULL DEFAULT '{}',
    base_version_id VARCHAR(36) NOT NULL DEFAULT '',
    token_hash VARCHAR(64) NOT NULL DEFAULT '',
    token_expires_at TIMESTAMP,
    token_consumed_at TIMESTAMP,
    confirm_comment TEXT NOT NULL DEFAULT '',
    applied_version_ids TEXT NOT NULL DEFAULT '[]',
    error TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agent_pending_tasks_scope
    ON agent_pending_tasks (tenant_id, user_id, status);
CREATE INDEX IF NOT EXISTS idx_agent_pending_tasks_session
    ON agent_pending_tasks (tenant_id, session_id);

CREATE TABLE IF NOT EXISTS doc_lineage (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    src_knowledge_id VARCHAR(36) NOT NULL,
    src_version_id VARCHAR(36) NOT NULL DEFAULT '',
    dst_knowledge_id VARCHAR(36) NOT NULL,
    dst_version_id VARCHAR(36) NOT NULL DEFAULT '',
    relation VARCHAR(24) NOT NULL,
    pending_task_id VARCHAR(36) NOT NULL DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_doc_lineage_dst ON doc_lineage (tenant_id, dst_knowledge_id);

ALTER TABLE doc_versions ADD COLUMN pending_task_id VARCHAR(36);
