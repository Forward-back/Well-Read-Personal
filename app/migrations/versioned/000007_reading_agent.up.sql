-- Reading agent P0: dual model roles, run phase, distilled memory, and
-- self-test candidates. Old state_version=2 runs keep working unchanged.

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS reading_extraction_model_id VARCHAR(36) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS reading_synthesis_model_id VARCHAR(36) NOT NULL DEFAULT '';

ALTER TABLE reading_runs
    ADD COLUMN IF NOT EXISTS extraction_model_id VARCHAR(36) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS synthesis_model_id VARCHAR(36) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS phase VARCHAR(16) NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS reading_run_memories (
    run_id      VARCHAR(36) PRIMARY KEY,
    tenant_id   BIGINT NOT NULL,
    knowledge   JSONB NOT NULL DEFAULT '{}'::jsonb,
    user_model  JSONB NOT NULL DEFAULT '{}'::jsonb,
    task        JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
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
    evidence     JSONB NOT NULL DEFAULT '[]'::jsonb,
    status       VARCHAR(16) NOT NULL DEFAULT 'candidate',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_self_tests_run
    ON reading_self_tests (run_id, created_at);
