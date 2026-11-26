DROP INDEX IF EXISTS idx_review_cards_suspended_until;
ALTER TABLE review_cards DROP COLUMN suspended_until;
