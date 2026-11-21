DROP INDEX IF EXISTS idx_user_backgrounds_user_id;
DROP TABLE IF EXISTS user_backgrounds;
ALTER TABLE users DROP COLUMN background_choice;
