CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    username TEXT NOT NULL COLLATE NOCASE UNIQUE,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    email TEXT NOT NULL DEFAULT '',
    google_subject TEXT
);
CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE email <> '';
CREATE UNIQUE INDEX idx_users_google_subject ON users(google_subject) WHERE google_subject IS NOT NULL;

CREATE TABLE user_settings (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    activity_visibility TEXT NOT NULL DEFAULT 'private'
        CHECK (activity_visibility IN ('private', 'instance')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    pushover_user_key_encrypted TEXT,
    pushover_notifications_enabled INTEGER NOT NULL DEFAULT 0
        CHECK (pushover_notifications_enabled IN (0, 1)),
    pushover_app_token_encrypted TEXT
);

CREATE TABLE sessions (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    last_used_at TEXT
);
CREATE INDEX idx_sessions_token_hash ON sessions(token_hash);

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
    catalog_updated_at TEXT,
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
CREATE INDEX idx_episodes_show_order ON episodes(show_id, season_number, episode_number);
CREATE INDEX idx_episodes_air_date ON episodes(air_date);

CREATE TABLE user_media (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_id TEXT NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('watchlist', 'watching', 'paused', 'dropped')),
    rating INTEGER CHECK (rating IS NULL OR rating BETWEEN 1 AND 5),
    notifications_enabled INTEGER NOT NULL DEFAULT 1
        CHECK (notifications_enabled IN (0, 1)),
    added_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    notifications_since TEXT,
    UNIQUE (user_id, media_id)
);
CREATE INDEX idx_user_media_user_status ON user_media(user_id, status);

CREATE TABLE plays (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_id TEXT REFERENCES media(id) ON DELETE CASCADE,
    episode_id TEXT REFERENCES episodes(id) ON DELETE CASCADE,
    watched_at TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'web',
    created_at TEXT NOT NULL,
    CHECK ((media_id IS NOT NULL AND episode_id IS NULL) OR
           (media_id IS NULL AND episode_id IS NOT NULL))
);
CREATE INDEX idx_plays_user_watched ON plays(user_id, watched_at DESC);
CREATE INDEX idx_plays_user_episode ON plays(user_id, episode_id);

CREATE TABLE activity_events (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('watch', 'rewatch', 'rating', 'bulk_watch')),
    play_id INTEGER REFERENCES plays(id) ON DELETE CASCADE,
    media_id TEXT REFERENCES media(id) ON DELETE CASCADE,
    episode_id TEXT REFERENCES episodes(id) ON DELETE CASCADE,
    rating INTEGER CHECK (rating IS NULL OR rating BETWEEN 1 AND 5),
    detail_json TEXT,
    occurred_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_activity_events_feed ON activity_events(occurred_at DESC, id);
CREATE INDEX idx_activity_events_created_at ON activity_events(created_at);

CREATE TABLE episode_ratings (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    episode_id TEXT NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    rating INTEGER CHECK (rating IS NULL OR rating BETWEEN 1 AND 5),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (user_id, episode_id)
);
CREATE INDEX idx_episode_ratings_user ON episode_ratings(user_id, updated_at DESC);

CREATE TABLE personal_api_tokens (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL,
    last_used_at TEXT,
    expires_at TEXT
);
CREATE INDEX idx_personal_api_tokens_user ON personal_api_tokens(user_id, created_at DESC);

CREATE TABLE notification_deliveries (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    episode_id TEXT NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    state TEXT NOT NULL CHECK (state IN ('sending', 'sent', 'failed')),
    attempted_at TEXT NOT NULL,
    sent_at TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 1 CHECK (attempt_count > 0),
    next_attempt_at TEXT,
    PRIMARY KEY (user_id, episode_id)
);
CREATE INDEX idx_notification_deliveries_state ON notification_deliveries(state, attempted_at);

CREATE TABLE oauth_clients (
    client_id TEXT PRIMARY KEY,
    client_name TEXT NOT NULL,
    redirect_uris_json TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE oauth_authorization_codes (
    code_hash TEXT PRIMARY KEY,
    client_id TEXT NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redirect_uri TEXT NOT NULL,
    code_challenge TEXT NOT NULL,
    scope TEXT NOT NULL,
    resource TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_oauth_codes_expiry ON oauth_authorization_codes(expires_at);

CREATE TABLE oauth_access_tokens (
    token_hash TEXT PRIMARY KEY,
    client_id TEXT NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scope TEXT NOT NULL,
    resource TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    revoked_at TEXT,
    created_at TEXT NOT NULL DEFAULT '',
    last_used_at TEXT
);
CREATE INDEX idx_oauth_tokens_user ON oauth_access_tokens(user_id, expires_at);

CREATE TABLE oauth_refresh_tokens (
    token_hash TEXT PRIMARY KEY,
    client_id TEXT NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scope TEXT NOT NULL,
    resource TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    revoked_at TEXT,
    created_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_oauth_refresh_user ON oauth_refresh_tokens(user_id, expires_at);

CREATE TABLE plex_webhooks (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL,
    last_used_at TEXT
);

CREATE TABLE plex_webhook_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
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
