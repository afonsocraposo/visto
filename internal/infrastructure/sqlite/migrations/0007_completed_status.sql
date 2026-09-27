CREATE TABLE user_media_new (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_id TEXT NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('watchlist', 'watching', 'paused', 'dropped', 'completed')),
    rating INTEGER CHECK (rating IS NULL OR rating BETWEEN 1 AND 5),
    notifications_enabled INTEGER NOT NULL DEFAULT 1 CHECK (notifications_enabled IN (0, 1)),
    added_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    notifications_since TEXT,
    UNIQUE (user_id, media_id)
);

INSERT INTO user_media_new(id,user_id,media_id,status,rating,notifications_enabled,added_at,updated_at,notifications_since)
SELECT um.id,um.user_id,um.media_id,
  CASE
    WHEN m.media_type='movie' AND EXISTS(SELECT 1 FROM plays p WHERE p.user_id=um.user_id AND p.media_id=m.id) THEN 'completed'
    WHEN m.media_type='movie' THEN 'watchlist'
    WHEN m.status IN ('Ended','Canceled','Cancelled')
      AND EXISTS(SELECT 1 FROM episodes e WHERE e.show_id=m.id AND e.season_number>0)
      AND (SELECT COUNT(*) FROM episodes e WHERE e.show_id=m.id AND e.season_number>0) >=
          COALESCE((SELECT SUM(s.episode_count) FROM seasons s WHERE s.show_id=m.id AND s.season_number>0),0)
      AND NOT EXISTS(SELECT 1 FROM episodes e WHERE e.show_id=m.id AND e.season_number>0
        AND NOT EXISTS(SELECT 1 FROM plays p WHERE p.user_id=um.user_id AND p.episode_id=e.id)) THEN 'completed'
    WHEN um.status='watchlist' AND EXISTS(SELECT 1 FROM episodes e JOIN plays p ON p.episode_id=e.id
      WHERE e.show_id=m.id AND p.user_id=um.user_id) THEN 'watching'
    ELSE um.status
  END,
  um.rating,um.notifications_enabled,um.added_at,um.updated_at,um.notifications_since
FROM user_media um JOIN media m ON m.id=um.media_id;

INSERT INTO user_media_new(id,user_id,media_id,status,rating,notifications_enabled,added_at,updated_at,notifications_since)
SELECT CAST(p.user_id AS TEXT)||':'||m.id,p.user_id,m.id,'completed',NULL,1,
  MIN(p.created_at),MAX(p.created_at),NULL
FROM plays p JOIN episodes watched ON watched.id=p.episode_id JOIN media m ON m.id=watched.show_id
WHERE m.status IN ('Ended','Canceled','Cancelled')
  AND NOT EXISTS(SELECT 1 FROM user_media_new um WHERE um.user_id=p.user_id AND um.media_id=m.id)
  AND EXISTS(SELECT 1 FROM episodes e WHERE e.show_id=m.id AND e.season_number>0)
  AND (SELECT COUNT(*) FROM episodes e WHERE e.show_id=m.id AND e.season_number>0) >=
      COALESCE((SELECT SUM(s.episode_count) FROM seasons s WHERE s.show_id=m.id AND s.season_number>0),0)
  AND NOT EXISTS(SELECT 1 FROM episodes e WHERE e.show_id=m.id AND e.season_number>0
    AND NOT EXISTS(SELECT 1 FROM plays watched_play WHERE watched_play.user_id=p.user_id AND watched_play.episode_id=e.id))
GROUP BY p.user_id,m.id;

DROP TABLE user_media;
ALTER TABLE user_media_new RENAME TO user_media;
CREATE INDEX idx_user_media_user_status ON user_media(user_id,status);
CREATE INDEX idx_user_media_paging_updated ON user_media(user_id,updated_at DESC,media_id);
