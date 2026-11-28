DROP INDEX IF EXISTS idx_doc_lineage_dst;
DROP TABLE IF EXISTS doc_lineage;
DROP INDEX IF EXISTS idx_agent_pending_tasks_session;
DROP INDEX IF EXISTS idx_agent_pending_tasks_scope;
DROP TABLE IF EXISTS agent_pending_tasks;
ALTER TABLE doc_versions DROP COLUMN IF EXISTS pending_task_id;
