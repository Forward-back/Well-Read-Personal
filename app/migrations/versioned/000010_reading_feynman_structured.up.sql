-- Structured Feynman retelling: keep the legacy text column for old clients
-- and reports, while storing per-angle answers and LLM feedback alongside it.

ALTER TABLE reading_feynman_attempts
    ADD COLUMN answers JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN angle_feedback JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN prompt_version INT NOT NULL DEFAULT 0;
