DROP TABLE IF EXISTS reading_understandings;
DROP TABLE IF EXISTS reading_drafts;

ALTER TABLE reading_terms DROP COLUMN IF EXISTS round;

ALTER TABLE reading_questions DROP COLUMN IF EXISTS judged_at;
ALTER TABLE reading_questions DROP COLUMN IF EXISTS judgment_reason;
ALTER TABLE reading_questions DROP COLUMN IF EXISTS judgment;
ALTER TABLE reading_questions DROP COLUMN IF EXISTS options;
