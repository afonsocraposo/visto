ALTER TABLE user_media ADD COLUMN season_alerts_enabled INTEGER NOT NULL DEFAULT 0
    CHECK (season_alerts_enabled IN (0, 1));

UPDATE user_media SET season_alerts_enabled=1, notifications_enabled=0,
    notifications_since=strftime('%Y-%m-%dT%H:%M:%fZ','now')
WHERE media_id LIKE 'tv:%' AND EXISTS (
    SELECT 1 FROM season_ready_alerts a JOIN seasons s ON s.id=a.season_id
    WHERE a.user_id=user_media.user_id AND s.show_id=user_media.media_id AND a.ready_at IS NULL
);
