CREATE TABLE plays_open (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_id TEXT REFERENCES media(id) ON DELETE CASCADE,
    episode_id TEXT REFERENCES episodes(id) ON DELETE CASCADE,
    watched_at TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'web',
    created_at TEXT NOT NULL,
    CHECK ((media_id IS NOT NULL AND episode_id IS NULL) OR
           (media_id IS NULL AND episode_id IS NOT NULL))
);
INSERT INTO plays_open(id,user_id,media_id,episode_id,watched_at,source,created_at)
SELECT id,user_id,media_id,episode_id,watched_at,source,created_at FROM plays;

CREATE TABLE activity_events_open (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('watch', 'rewatch', 'rating', 'bulk_watch')),
    play_id TEXT REFERENCES plays_open(id) ON DELETE CASCADE,
    media_id TEXT REFERENCES media(id) ON DELETE CASCADE,
    episode_id TEXT REFERENCES episodes(id) ON DELETE CASCADE,
    rating INTEGER CHECK (rating IS NULL OR rating BETWEEN 1 AND 5),
    detail_json TEXT,
    occurred_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);
INSERT INTO activity_events_open(id,user_id,kind,play_id,media_id,episode_id,rating,detail_json,occurred_at,created_at)
SELECT id,user_id,kind,play_id,media_id,episode_id,rating,detail_json,occurred_at,created_at FROM activity_events;

DROP TABLE activity_events;
DROP TABLE plays;
ALTER TABLE plays_open RENAME TO plays;
ALTER TABLE activity_events_open RENAME TO activity_events;

CREATE INDEX idx_plays_user_watched ON plays(user_id, watched_at DESC);
CREATE INDEX idx_plays_user_episode ON plays(user_id, episode_id);
CREATE INDEX idx_activity_events_feed ON activity_events(occurred_at DESC, id);
