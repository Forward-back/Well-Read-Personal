-- SQLite variant of 000009_reading_reader_layer.

CREATE TABLE IF NOT EXISTS reading_feynman_attempts (
    id              VARCHAR(36) PRIMARY KEY,
    run_id          VARCHAR(36) NOT NULL,
    tenant_id       BIGINT NOT NULL,
    section_id      VARCHAR(64) NOT NULL DEFAULT '',
    text            TEXT NOT NULL DEFAULT '',
    score           INTEGER NOT NULL DEFAULT 0,
    coverage        INTEGER NOT NULL DEFAULT 0,
    accuracy        INTEGER NOT NULL DEFAULT 0,
    evidence_score  INTEGER NOT NULL DEFAULT 0,
    summary         TEXT NOT NULL DEFAULT '',
    omissions       TEXT NOT NULL DEFAULT '[]',
    errors          TEXT NOT NULL DEFAULT '[]',
    evidence        TEXT NOT NULL DEFAULT '[]',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_feynman_run
    ON reading_feynman_attempts (run_id, section_id, created_at);

CREATE TABLE IF NOT EXISTS reading_annotations (
    id           VARCHAR(36) PRIMARY KEY,
    run_id       VARCHAR(36) NOT NULL,
    tenant_id    BIGINT NOT NULL,
    section_id   VARCHAR(64) NOT NULL DEFAULT '',
    knowledge_id VARCHAR(36) NOT NULL DEFAULT '',
    chunk_id     VARCHAR(36) NOT NULL DEFAULT '',
    quote        TEXT NOT NULL DEFAULT '',
    comment      TEXT NOT NULL DEFAULT '',
    status       VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_annotations_run
    ON reading_annotations (run_id, section_id, created_at);
