-- Review V3: four-tier rating. Existing sessions keep easy_count = 0; the
-- history is not rewritten.

ALTER TABLE review_sessions ADD COLUMN easy_count INTEGER NOT NULL DEFAULT 0;
