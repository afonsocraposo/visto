CREATE TABLE plays_new (
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
INSERT INTO plays_new(id,user_id,media_id,episode_id,watched_at,source,created_at)
SELECT id,user_id,media_id,episode_id,watched_at,source,created_at FROM plays;

CREATE TABLE activity_events_new (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('watch', 'rewatch', 'rating', 'bulk_watch')),
    play_id TEXT REFERENCES plays_new(id) ON DELETE CASCADE,
    media_id TEXT REFERENCES media(id) ON DELETE CASCADE,
    episode_id TEXT REFERENCES episodes(id) ON DELETE CASCADE,
    rating INTEGER CHECK (rating IS NULL OR rating BETWEEN 1 AND 5),
    detail_json TEXT,
    occurred_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);
INSERT INTO activity_events_new(id,user_id,kind,play_id,media_id,episode_id,rating,detail_json,occurred_at,created_at)
SELECT id,user_id,kind,play_id,media_id,episode_id,rating,detail_json,occurred_at,created_at FROM activity_events;

DROP TABLE activity_events;
DROP TABLE plays;
ALTER TABLE plays_new RENAME TO plays;
ALTER TABLE activity_events_new RENAME TO activity_events;

CREATE INDEX idx_plays_user_watched ON plays(user_id, watched_at DESC);
CREATE INDEX idx_plays_user_episode ON plays(user_id, episode_id);
CREATE INDEX idx_activity_events_feed ON activity_events(occurred_at DESC, id);

CREATE TABLE plex_webhooks (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL,
    last_used_at TEXT
);

CREATE TABLE plex_webhook_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    fingerprint TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('processing', 'synced', 'skipped', 'failed')),
    title TEXT,
    media_type TEXT,
    tmdb_id INTEGER,
    message TEXT,
    occurred_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (user_id, fingerprint)
);
CREATE INDEX idx_plex_webhook_events_recent ON plex_webhook_events(user_id, created_at DESC, id DESC);
