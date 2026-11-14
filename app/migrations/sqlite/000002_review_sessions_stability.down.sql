DROP INDEX IF EXISTS idx_review_logs_session;
DROP TABLE IF EXISTS review_sessions;
ALTER TABLE review_logs DROP COLUMN stability_after;
ALTER TABLE review_logs DROP COLUMN stability_before;
ALTER TABLE review_logs DROP COLUMN session_id;
ALTER TABLE review_cards DROP COLUMN forget_streak;
ALTER TABLE review_cards DROP COLUMN stability;
