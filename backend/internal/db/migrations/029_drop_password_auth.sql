-- Auth runs on Firebase ID tokens, so the password/JWT schema from the
-- pre-Firebase flow is dead: users.password_hash only ever held '' and
-- refresh_tokens is never read or written.

ALTER TABLE users DROP COLUMN IF EXISTS password_hash;

DROP TABLE IF EXISTS refresh_tokens;
