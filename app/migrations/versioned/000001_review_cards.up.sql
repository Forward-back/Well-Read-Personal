-- Review groups, cards, sources, and review logs.
-- Additive migration; 000000_init stays frozen.

CREATE TABLE IF NOT EXISTS review_groups (
    id          VARCHAR(64) PRIMARY KEY,
    tenant_id   BIGINT NOT NULL,
    parent_id   VARCHAR(64) NOT NULL DEFAULT '',
    name        VARCHAR(200) NOT NULL,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    is_default  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_review_groups_tenant_parent
    ON review_groups (tenant_id, parent_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_review_groups_tenant_default
    ON review_groups (tenant_id, is_default);

CREATE TABLE IF NOT EXISTS review_cards (
    id                VARCHAR(64) PRIMARY KEY,
    tenant_id         BIGINT NOT NULL,
    group_id          VARCHAR(64) NOT NULL,
    card_type         VARCHAR(20) NOT NULL DEFAULT 'qa',
    question          TEXT NOT NULL,
    answer            TEXT NOT NULL,
    explanation       TEXT NOT NULL DEFAULT '',
    tags              JSONB NOT NULL DEFAULT '[]'::jsonb,
    status            VARCHAR(20) NOT NULL DEFAULT 'active',
    review_level      INTEGER NOT NULL DEFAULT 0,
    due_at            TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    lapses            INTEGER NOT NULL DEFAULT 0,
    review_count      INTEGER NOT NULL DEFAULT 0,
    question_hash     VARCHAR(64) NOT NULL,
    last_reviewed_at  TIMESTAMPTZ,
    source_updated    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at        TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_review_cards_queue
    ON review_cards (tenant_id, status, due_at, deleted_at);
CREATE INDEX IF NOT EXISTS idx_review_cards_group
    ON review_cards (tenant_id, group_id, status, deleted_at);
CREATE INDEX IF NOT EXISTS idx_review_cards_hash
    ON review_cards (tenant_id, question_hash);

CREATE TABLE IF NOT EXISTS review_card_sources (
    id             VARCHAR(64) PRIMARY KEY,
    tenant_id      BIGINT NOT NULL,
    card_id        VARCHAR(64) NOT NULL,
    source_type    VARCHAR(24) NOT NULL,
    knowledge_id   VARCHAR(64) NOT NULL DEFAULT '',
    chunk_id       VARCHAR(64) NOT NULL DEFAULT '',
    message_id     VARCHAR(64) NOT NULL DEFAULT '',
    path           VARCHAR(1024) NOT NULL DEFAULT '',
    symbol         VARCHAR(256) NOT NULL DEFAULT '',
    start_line     INTEGER NOT NULL DEFAULT 0,
    end_line       INTEGER NOT NULL DEFAULT 0,
    content_hash   VARCHAR(64) NOT NULL DEFAULT '',
    excerpt        TEXT NOT NULL DEFAULT '',
    is_primary     BOOLEAN NOT NULL DEFAULT FALSE,
    status         VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_review_sources_card
    ON review_card_sources (tenant_id, card_id, status);
CREATE INDEX IF NOT EXISTS idx_review_sources_knowledge
    ON review_card_sources (tenant_id, knowledge_id);

CREATE TABLE IF NOT EXISTS review_logs (
    id                 VARCHAR(64) PRIMARY KEY,
    tenant_id          BIGINT NOT NULL,
    card_id            VARCHAR(64) NOT NULL,
    rating             VARCHAR(20) NOT NULL,
    level_before       INTEGER NOT NULL,
    level_after        INTEGER NOT NULL,
    interval_days      INTEGER NOT NULL,
    reviewed_at        TIMESTAMPTZ NOT NULL,
    client_request_id  VARCHAR(64) NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_review_logs_request
    ON review_logs (tenant_id, client_request_id);
CREATE INDEX IF NOT EXISTS idx_review_logs_card
    ON review_logs (tenant_id, card_id, reviewed_at);
CREATE INDEX IF NOT EXISTS idx_review_logs_day
    ON review_logs (tenant_id, reviewed_at);
