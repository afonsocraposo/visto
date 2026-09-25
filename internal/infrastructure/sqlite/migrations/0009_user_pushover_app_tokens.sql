ALTER TABLE user_settings ADD COLUMN pushover_app_token_encrypted TEXT;

-- Every account now supplies its own app token and must explicitly opt in again.
UPDATE user_settings SET pushover_notifications_enabled=0 WHERE pushover_notifications_enabled=1;
