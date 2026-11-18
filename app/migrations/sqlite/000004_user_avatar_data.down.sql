-- Rollback of uploaded avatar storage.

ALTER TABLE users DROP COLUMN avatar_data;
