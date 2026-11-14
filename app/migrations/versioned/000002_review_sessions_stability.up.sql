-- Review V2: adaptive stability, session tracking, and forget streak.

ALTER TABLE review_cards
    ADD COLUMN IF NOT EXISTS stability DOUBLE PRECISION NOT NULL DEFAULT 10;
ALTER TABLE review_cards
    ADD COLUMN IF NOT EXISTS forget_streak INTEGER NOT NULL DEFAULT 0;

-- Existing cards keep their current schedule meaning: derive stability from
-- the legacy L0-L6 interval so due dates and review history stay coherent.
UPDATE review_cards
SET stability = CASE review_level
    WHEN 0 THEN 9.49
    WHEN 1 THEN 28.48
    WHEN 2 THEN 66.44
    WHEN 3 THEN 142.37
    WHEN 4 THEN 284.75
    WHEN 5 THEN 569.49
    WHEN 6 THEN 1138.98
    ELSE 9.49
END
WHERE stability = 10;

ALTER TABLE review_logs
    ADD COLUMN IF NOT EXISTS session_id VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE review_logs
    ADD COLUMN IF NOT EXISTS stability_before DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE review_logs
    ADD COLUMN IF NOT EXISTS stability_after DOUBLE PRECISION NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_review_logs_session
    ON review_logs (tenant_id, session_id, reviewed_at);

CREATE TABLE IF NOT EXISTS review_sessions (
    id                VARCHAR(64) PRIMARY KEY,
    tenant_id         BIGINT NOT NULL,
    mode              VARCHAR(20) NOT NULL,
    status            VARCHAR(20) NOT NULL DEFAULT 'active',
    group_id          VARCHAR(64) NOT NULL DEFAULT '',
    planned_card_ids  JSONB NOT NULL DEFAULT '[]'::jsonb,
    planned_count     INTEGER NOT NULL DEFAULT 0,
    completed_count   INTEGER NOT NULL DEFAULT 0,
    remembered_count  INTEGER NOT NULL DEFAULT 0,
    fuzzy_count       INTEGER NOT NULL DEFAULT 0,
    forgot_count      INTEGER NOT NULL DEFAULT 0,
    review_day        TIMESTAMPTZ NOT NULL,
    started_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_review_sessions_active
    ON review_sessions (tenant_id, status, review_day);
CREATE INDEX IF NOT EXISTS idx_review_sessions_history
    ON review_sessions (tenant_id, started_at DESC);
