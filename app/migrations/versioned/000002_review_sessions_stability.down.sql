ALTER TABLE review_logs DROP COLUMN IF EXISTS stability_after;
ALTER TABLE review_logs DROP COLUMN IF EXISTS stability_before;
DROP INDEX IF EXISTS idx_review_logs_session;
ALTER TABLE review_logs DROP COLUMN IF EXISTS session_id;
DROP TABLE IF EXISTS review_sessions;
ALTER TABLE review_cards DROP COLUMN IF EXISTS forget_streak;
ALTER TABLE review_cards DROP COLUMN IF EXISTS stability;
