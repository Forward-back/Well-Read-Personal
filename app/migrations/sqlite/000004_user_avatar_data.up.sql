-- Personal avatar upload. The legacy users.avatar column keeps its URL
-- semantics; uploaded avatars are stored as small data URLs in TEXT.

ALTER TABLE users ADD COLUMN avatar_data TEXT;
