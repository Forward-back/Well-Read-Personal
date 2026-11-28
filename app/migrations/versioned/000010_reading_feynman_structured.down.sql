ALTER TABLE reading_feynman_attempts
    DROP COLUMN IF EXISTS answers,
    DROP COLUMN IF EXISTS angle_feedback,
    DROP COLUMN IF EXISTS prompt_version;
