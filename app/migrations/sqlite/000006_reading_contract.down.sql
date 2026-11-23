DROP TABLE IF EXISTS reading_understandings;
DROP TABLE IF EXISTS reading_drafts;

ALTER TABLE reading_terms DROP COLUMN round;

ALTER TABLE reading_questions DROP COLUMN judged_at;
ALTER TABLE reading_questions DROP COLUMN judgment_reason;
ALTER TABLE reading_questions DROP COLUMN judgment;
ALTER TABLE reading_questions DROP COLUMN options;
