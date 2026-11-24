-- SQLite variant of the reading agent P0 contract. JSON values are stored as
-- TEXT and encoded/decoded by the application.

ALTER TABLE tenants ADD COLUMN reading_extraction_model_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE tenants ADD COLUMN reading_synthesis_model_id VARCHAR(36) NOT NULL DEFAULT '';

ALTER TABLE reading_runs ADD COLUMN extraction_model_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE reading_runs ADD COLUMN synthesis_model_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE reading_runs ADD COLUMN phase VARCHAR(16) NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS reading_run_memories (
    run_id      VARCHAR(36) PRIMARY KEY,
    tenant_id   BIGINT NOT NULL,
    knowledge   TEXT NOT NULL DEFAULT '{}',
    user_model  TEXT NOT NULL DEFAULT '{}',
    task        TEXT NOT NULL DEFAULT '{}',
    updated_at  DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_reading_run_memories_tenant
    ON reading_run_memories (tenant_id);

CREATE TABLE IF NOT EXISTS reading_self_tests (
    id           VARCHAR(36) PRIMARY KEY,
    run_id       VARCHAR(36) NOT NULL,
    tenant_id    BIGINT NOT NULL,
    section_id   VARCHAR(64) NOT NULL DEFAULT '',
    question     TEXT NOT NULL DEFAULT '',
    answer       TEXT NOT NULL DEFAULT '',
    explanation  TEXT NOT NULL DEFAULT '',
    evidence     TEXT NOT NULL DEFAULT '[]',
    status       VARCHAR(16) NOT NULL DEFAULT 'candidate',
    created_at   DATETIME NOT NULL,
    updated_at   DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_reading_self_tests_run
    ON reading_self_tests (run_id, created_at);
