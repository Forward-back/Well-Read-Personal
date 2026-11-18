-- Rollback of uploaded avatar storage.

ALTER TABLE users DROP COLUMN IF EXISTS avatar_data;
