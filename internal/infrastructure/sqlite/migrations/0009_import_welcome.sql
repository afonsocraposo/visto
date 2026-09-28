ALTER TABLE user_settings ADD COLUMN import_welcome_seen_at TEXT;
UPDATE user_settings SET import_welcome_seen_at = CURRENT_TIMESTAMP;
