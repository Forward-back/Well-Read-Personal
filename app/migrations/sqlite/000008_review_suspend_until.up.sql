-- SQLite variant of 000008_review_suspend_until.

ALTER TABLE review_cards ADD COLUMN suspended_until DATETIME;

CREATE INDEX IF NOT EXISTS idx_review_cards_suspended_until
    ON review_cards (tenant_id, status, suspended_until);
