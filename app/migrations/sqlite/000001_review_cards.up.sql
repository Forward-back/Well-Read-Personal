-- Review groups, cards, sources, and review logs.
-- Additive migration; 000000_init stays frozen.

CREATE TABLE IF NOT EXISTS review_groups (
    id          TEXT PRIMARY KEY,
    tenant_id   INTEGER NOT NULL,
    parent_id   TEXT NOT NULL DEFAULT '',
    name        TEXT NOT NULL,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    is_default  INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at  DATETIME
);

CREATE INDEX IF NOT EXISTS idx_review_groups_tenant_parent
    ON review_groups (tenant_id, parent_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_review_groups_tenant_default
    ON review_groups (tenant_id, is_default);

CREATE TABLE IF NOT EXISTS review_cards (
    id                TEXT PRIMARY KEY,
    tenant_id         INTEGER NOT NULL,
    group_id          TEXT NOT NULL,
    card_type         TEXT NOT NULL DEFAULT 'qa',
    question          TEXT NOT NULL,
    answer            TEXT NOT NULL,
    explanation       TEXT NOT NULL DEFAULT '',
    tags              TEXT NOT NULL DEFAULT '[]',
    status            TEXT NOT NULL DEFAULT 'active',
    review_level      INTEGER NOT NULL DEFAULT 0,
    due_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    lapses            INTEGER NOT NULL DEFAULT 0,
    review_count      INTEGER NOT NULL DEFAULT 0,
    question_hash     TEXT NOT NULL,
    last_reviewed_at  DATETIME,
    source_updated    INTEGER NOT NULL DEFAULT 0,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at        DATETIME
);

CREATE INDEX IF NOT EXISTS idx_review_cards_queue
    ON review_cards (tenant_id, status, due_at, deleted_at);
CREATE INDEX IF NOT EXISTS idx_review_cards_group
    ON review_cards (tenant_id, group_id, status, deleted_at);
CREATE INDEX IF NOT EXISTS idx_review_cards_hash
    ON review_cards (tenant_id, question_hash);

CREATE TABLE IF NOT EXISTS review_card_sources (
    id             TEXT PRIMARY KEY,
    tenant_id      INTEGER NOT NULL,
    card_id        TEXT NOT NULL,
    source_type    TEXT NOT NULL,
    knowledge_id   TEXT NOT NULL DEFAULT '',
    chunk_id       TEXT NOT NULL DEFAULT '',
    message_id     TEXT NOT NULL DEFAULT '',
    path           TEXT NOT NULL DEFAULT '',
    symbol         TEXT NOT NULL DEFAULT '',
    start_line     INTEGER NOT NULL DEFAULT 0,
    end_line       INTEGER NOT NULL DEFAULT 0,
    content_hash   TEXT NOT NULL DEFAULT '',
    excerpt        TEXT NOT NULL DEFAULT '',
    is_primary     INTEGER NOT NULL DEFAULT 0,
    status         TEXT NOT NULL DEFAULT 'active',
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_review_sources_card
    ON review_card_sources (tenant_id, card_id, status);
CREATE INDEX IF NOT EXISTS idx_review_sources_knowledge
    ON review_card_sources (tenant_id, knowledge_id);

CREATE TABLE IF NOT EXISTS review_logs (
    id                 TEXT PRIMARY KEY,
    tenant_id          INTEGER NOT NULL,
    card_id            TEXT NOT NULL,
    rating             TEXT NOT NULL,
    level_before       INTEGER NOT NULL,
    level_after        INTEGER NOT NULL,
    interval_days      INTEGER NOT NULL,
    reviewed_at        DATETIME NOT NULL,
    client_request_id  TEXT NOT NULL,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_review_logs_request
    ON review_logs (tenant_id, client_request_id);
CREATE INDEX IF NOT EXISTS idx_review_logs_card
    ON review_logs (tenant_id, card_id, reviewed_at);
CREATE INDEX IF NOT EXISTS idx_review_logs_day
    ON review_logs (tenant_id, reviewed_at);
