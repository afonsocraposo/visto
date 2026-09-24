ALTER TABLE oauth_access_tokens ADD COLUMN created_at TEXT NOT NULL DEFAULT '';
ALTER TABLE oauth_access_tokens ADD COLUMN last_used_at TEXT;

ALTER TABLE oauth_refresh_tokens ADD COLUMN created_at TEXT NOT NULL DEFAULT '';
