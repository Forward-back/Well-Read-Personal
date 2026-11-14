-- Review V2: adaptive stability, session tracking, and forget streak.

ALTER TABLE review_cards ADD COLUMN stability REAL NOT NULL DEFAULT 10;
ALTER TABLE review_cards ADD COLUMN forget_streak INTEGER NOT NULL DEFAULT 0;

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

ALTER TABLE review_logs ADD COLUMN session_id TEXT NOT NULL DEFAULT '';
ALTER TABLE review_logs ADD COLUMN stability_before REAL NOT NULL DEFAULT 0;
ALTER TABLE review_logs ADD COLUMN stability_after REAL NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_review_logs_session
    ON review_logs (tenant_id, session_id, reviewed_at);

CREATE TABLE IF NOT EXISTS review_sessions (
    id                TEXT PRIMARY KEY,
    tenant_id         INTEGER NOT NULL,
    mode              TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'active',
    group_id          TEXT NOT NULL DEFAULT '',
    planned_card_ids  TEXT NOT NULL DEFAULT '[]',
    planned_count     INTEGER NOT NULL DEFAULT 0,
    completed_count   INTEGER NOT NULL DEFAULT 0,
    remembered_count  INTEGER NOT NULL DEFAULT 0,
    fuzzy_count       INTEGER NOT NULL DEFAULT 0,
    forgot_count      INTEGER NOT NULL DEFAULT 0,
    review_day        DATETIME NOT NULL,
    started_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at      DATETIME,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_review_sessions_active
    ON review_sessions (tenant_id, status, review_day);
CREATE INDEX IF NOT EXISTS idx_review_sessions_history
    ON review_sessions (tenant_id, started_at DESC);
