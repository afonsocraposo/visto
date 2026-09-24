CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL COLLATE NOCASE UNIQUE,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE user_settings (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    activity_visibility TEXT NOT NULL DEFAULT 'private'
        CHECK (activity_visibility IN ('private', 'instance')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    last_used_at TEXT
);

CREATE TABLE media (
    id TEXT PRIMARY KEY,
    media_type TEXT NOT NULL CHECK (media_type IN ('movie', 'tv')),
    tmdb_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    original_title TEXT,
    overview TEXT,
    release_date TEXT,
    status TEXT,
    poster_path TEXT,
    backdrop_path TEXT,
    original_language TEXT,
    raw_metadata TEXT,
    metadata_updated_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (media_type, tmdb_id)
);

CREATE TABLE seasons (
    id TEXT PRIMARY KEY,
    show_id TEXT NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    tmdb_id INTEGER,
    season_number INTEGER NOT NULL,
    name TEXT,
    overview TEXT,
    poster_path TEXT,
    air_date TEXT,
    episode_count INTEGER,
    UNIQUE (show_id, season_number)
);

CREATE TABLE episodes (
    id TEXT PRIMARY KEY,
    show_id TEXT NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    season_id TEXT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    tmdb_id INTEGER,
    season_number INTEGER NOT NULL,
    episode_number INTEGER NOT NULL,
    name TEXT,
    overview TEXT,
    air_date TEXT,
    runtime INTEGER,
    still_path TEXT,
    raw_metadata TEXT,
    metadata_updated_at TEXT,
    UNIQUE (show_id, season_number, episode_number)
);

CREATE TABLE user_media (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_id TEXT NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('watchlist', 'watching', 'paused', 'dropped')),
    rating INTEGER CHECK (rating IS NULL OR rating BETWEEN 1 AND 5),
    notifications_enabled INTEGER NOT NULL DEFAULT 1
        CHECK (notifications_enabled IN (0, 1)),
    added_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (user_id, media_id)
);

CREATE TABLE plays (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_id TEXT REFERENCES media(id) ON DELETE CASCADE,
    episode_id TEXT REFERENCES episodes(id) ON DELETE CASCADE,
    watched_at TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'web' CHECK (source IN ('web', 'api', 'mcp', 'import')),
    created_at TEXT NOT NULL,
    CHECK ((media_id IS NOT NULL AND episode_id IS NULL) OR
           (media_id IS NULL AND episode_id IS NOT NULL))
);

CREATE TABLE activity_events (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('watch', 'rewatch', 'rating', 'bulk_watch')),
    play_id TEXT REFERENCES plays(id) ON DELETE CASCADE,
    media_id TEXT REFERENCES media(id) ON DELETE CASCADE,
    episode_id TEXT REFERENCES episodes(id) ON DELETE CASCADE,
    rating INTEGER CHECK (rating IS NULL OR rating BETWEEN 1 AND 5),
    detail_json TEXT,
    occurred_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX idx_user_media_user_status ON user_media(user_id, status);
CREATE INDEX idx_plays_user_watched ON plays(user_id, watched_at DESC);
CREATE INDEX idx_plays_user_episode ON plays(user_id, episode_id);
CREATE INDEX idx_episodes_show_order ON episodes(show_id, season_number, episode_number);
CREATE INDEX idx_episodes_air_date ON episodes(air_date);
CREATE INDEX idx_activity_events_feed ON activity_events(occurred_at DESC, id);
