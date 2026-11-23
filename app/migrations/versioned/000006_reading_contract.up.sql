-- Interactive reading workbench contract: persisted drafts, candidate answers
-- with judgment, and per-round understanding decisions. Status-only features
-- such as abandoned runs and section jumps use existing columns.

CREATE TABLE IF NOT EXISTS reading_drafts (
    id            VARCHAR(36) PRIMARY KEY,
    tenant_id     BIGINT NOT NULL,
    user_id       VARCHAR(36) NOT NULL,
    name          VARCHAR(120) NOT NULL,
    document_ids  JSONB NOT NULL DEFAULT '[]'::jsonb,
    depth         VARCHAR(16) NOT NULL DEFAULT 'standard',
    goal          TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_drafts_user
    ON reading_drafts (tenant_id, user_id, updated_at DESC);

ALTER TABLE reading_questions
    ADD COLUMN IF NOT EXISTS options JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS judgment VARCHAR(16) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS judgment_reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS judged_at TIMESTAMPTZ;

ALTER TABLE reading_terms
    ADD COLUMN IF NOT EXISTS round INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS reading_understandings (
    id          VARCHAR(36) PRIMARY KEY,
    run_id      VARCHAR(36) NOT NULL,
    tenant_id   BIGINT NOT NULL,
    section_id  VARCHAR(64) NOT NULL DEFAULT '',
    round       INTEGER NOT NULL DEFAULT 0,
    text        TEXT NOT NULL DEFAULT '',
    decision    VARCHAR(16) NOT NULL DEFAULT 'pending',
    correction  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (run_id, section_id, round)
);

CREATE INDEX IF NOT EXISTS idx_reading_understandings_run
    ON reading_understandings (run_id, round);
