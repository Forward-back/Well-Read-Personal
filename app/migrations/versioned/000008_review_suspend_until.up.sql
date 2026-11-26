-- Review card suspension until a date. Suspended cards stay out of queues;
-- expired suspensions auto-resume with the due date shifted to the resume day.

ALTER TABLE review_cards
    ADD COLUMN IF NOT EXISTS suspended_until TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_review_cards_suspended_until
    ON review_cards (tenant_id, status, suspended_until);
