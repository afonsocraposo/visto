ALTER TABLE user_settings ADD COLUMN pushover_user_key_encrypted TEXT;
ALTER TABLE user_settings ADD COLUMN pushover_notifications_enabled INTEGER NOT NULL DEFAULT 0
    CHECK (pushover_notifications_enabled IN (0, 1));

ALTER TABLE user_media ADD COLUMN notifications_since TEXT;
UPDATE user_media SET notifications_since=updated_at WHERE status='watching';

CREATE TABLE notification_deliveries (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    episode_id TEXT NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    state TEXT NOT NULL CHECK (state IN ('sending', 'sent', 'failed')),
    attempted_at TEXT NOT NULL,
    sent_at TEXT,
    PRIMARY KEY (user_id, episode_id)
);

CREATE INDEX idx_notification_deliveries_state ON notification_deliveries(state, attempted_at);
