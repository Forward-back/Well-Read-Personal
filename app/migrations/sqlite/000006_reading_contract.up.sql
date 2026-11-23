-- SQLite variant of the interactive reading workbench contract. JSON values
-- are stored as TEXT and encoded/decoded by the application.

CREATE TABLE IF NOT EXISTS reading_drafts (
    id            VARCHAR(36) PRIMARY KEY,
    tenant_id     BIGINT NOT NULL,
    user_id       VARCHAR(36) NOT NULL,
    name          VARCHAR(120) NOT NULL,
    document_ids  TEXT NOT NULL DEFAULT '[]',
    depth         VARCHAR(16) NOT NULL DEFAULT 'standard',
    goal          TEXT NOT NULL DEFAULT '',
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_reading_drafts_user
    ON reading_drafts (tenant_id, user_id, updated_at DESC);

ALTER TABLE reading_questions ADD COLUMN options TEXT NOT NULL DEFAULT '[]';
ALTER TABLE reading_questions ADD COLUMN judgment VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE reading_questions ADD COLUMN judgment_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE reading_questions ADD COLUMN judged_at DATETIME;

ALTER TABLE reading_terms ADD COLUMN round INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS reading_understandings (
    id          VARCHAR(36) PRIMARY KEY,
    run_id      VARCHAR(36) NOT NULL,
    tenant_id   BIGINT NOT NULL,
    section_id  VARCHAR(64) NOT NULL DEFAULT '',
    round       INTEGER NOT NULL DEFAULT 0,
    text        TEXT NOT NULL DEFAULT '',
    decision    VARCHAR(16) NOT NULL DEFAULT 'pending',
    correction  TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL,
    UNIQUE (run_id, section_id, round)
);

CREATE INDEX IF NOT EXISTS idx_reading_understandings_run
    ON reading_understandings (run_id, round);
