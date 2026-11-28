-- SQLite variant of 000010_reading_feynman_structured.

ALTER TABLE reading_feynman_attempts ADD COLUMN answers TEXT NOT NULL DEFAULT '[]';
ALTER TABLE reading_feynman_attempts ADD COLUMN angle_feedback TEXT NOT NULL DEFAULT '[]';
ALTER TABLE reading_feynman_attempts ADD COLUMN prompt_version INTEGER NOT NULL DEFAULT 0;
