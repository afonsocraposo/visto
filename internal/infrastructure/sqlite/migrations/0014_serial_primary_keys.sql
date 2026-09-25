-- visto:migration foreign-keys=off

CREATE TEMP TABLE serial_user_ids (
    old_id TEXT PRIMARY KEY,
    new_id INTEGER NOT NULL UNIQUE
);
INSERT INTO serial_user_ids(old_id,new_id)
SELECT id,ROW_NUMBER() OVER (ORDER BY id) FROM users;

CREATE TEMP TABLE serial_users AS
SELECT serial_user_ids.new_id AS id,users.username,users.display_name,users.password_hash,
       users.role,users.created_at,users.updated_at,users.email,users.google_subject
FROM users JOIN serial_user_ids ON serial_user_ids.old_id=users.id;

UPDATE user_settings SET user_id=CAST((SELECT new_id FROM serial_user_ids WHERE old_id=user_settings.user_id) AS TEXT);
UPDATE sessions SET user_id=CAST((SELECT new_id FROM serial_user_ids WHERE old_id=sessions.user_id) AS TEXT);
UPDATE user_media
SET id=CAST((SELECT new_id FROM serial_user_ids WHERE old_id=user_media.user_id) AS TEXT)||':'||media_id,
    user_id=CAST((SELECT new_id FROM serial_user_ids WHERE old_id=user_media.user_id) AS TEXT);
UPDATE plays SET user_id=CAST((SELECT new_id FROM serial_user_ids WHERE old_id=plays.user_id) AS TEXT);
UPDATE activity_events SET user_id=CAST((SELECT new_id FROM serial_user_ids WHERE old_id=activity_events.user_id) AS TEXT);
UPDATE episode_ratings SET user_id=CAST((SELECT new_id FROM serial_user_ids WHERE old_id=episode_ratings.user_id) AS TEXT);
UPDATE personal_api_tokens SET user_id=CAST((SELECT new_id FROM serial_user_ids WHERE old_id=personal_api_tokens.user_id) AS TEXT);
UPDATE notification_deliveries SET user_id=CAST((SELECT new_id FROM serial_user_ids WHERE old_id=notification_deliveries.user_id) AS TEXT);
UPDATE oauth_authorization_codes SET user_id=CAST((SELECT new_id FROM serial_user_ids WHERE old_id=oauth_authorization_codes.user_id) AS TEXT);
UPDATE oauth_access_tokens SET user_id=CAST((SELECT new_id FROM serial_user_ids WHERE old_id=oauth_access_tokens.user_id) AS TEXT);
UPDATE oauth_refresh_tokens SET user_id=CAST((SELECT new_id FROM serial_user_ids WHERE old_id=oauth_refresh_tokens.user_id) AS TEXT);
UPDATE plex_webhooks SET user_id=CAST((SELECT new_id FROM serial_user_ids WHERE old_id=plex_webhooks.user_id) AS TEXT);
UPDATE plex_webhook_events SET user_id=CAST((SELECT new_id FROM serial_user_ids WHERE old_id=plex_webhook_events.user_id) AS TEXT);

DROP TABLE users;
CREATE TABLE users_serial (
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
INSERT INTO users_serial(id,username,display_name,password_hash,role,created_at,updated_at,email,google_subject)
SELECT id,username,display_name,password_hash,role,created_at,updated_at,email,google_subject
FROM serial_users ORDER BY id;
ALTER TABLE users_serial RENAME TO users;
CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE email <> '';
CREATE UNIQUE INDEX idx_users_google_subject ON users(google_subject) WHERE google_subject IS NOT NULL;

CREATE TABLE sessions_serial (
    id INTEGER PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    last_used_at TEXT
);
INSERT INTO sessions_serial(user_id,token_hash,expires_at,created_at,last_used_at)
SELECT user_id,token_hash,expires_at,created_at,last_used_at FROM sessions;
DROP TABLE sessions;
ALTER TABLE sessions_serial RENAME TO sessions;
CREATE INDEX idx_sessions_token_hash ON sessions(token_hash);

CREATE TABLE personal_api_tokens_serial (
    id INTEGER PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL,
    last_used_at TEXT,
    expires_at TEXT
);
INSERT INTO personal_api_tokens_serial(user_id,name,token_hash,created_at,last_used_at,expires_at)
SELECT user_id,name,token_hash,created_at,last_used_at,expires_at FROM personal_api_tokens;
DROP TABLE personal_api_tokens;
ALTER TABLE personal_api_tokens_serial RENAME TO personal_api_tokens;
CREATE INDEX idx_personal_api_tokens_user ON personal_api_tokens(user_id, created_at DESC);

CREATE TEMP TABLE serial_play_ids (
    old_id TEXT PRIMARY KEY,
    new_id INTEGER NOT NULL UNIQUE
);
INSERT INTO serial_play_ids(old_id,new_id)
SELECT id,ROW_NUMBER() OVER (ORDER BY id) FROM plays;
CREATE TEMP TABLE serial_activity_ids (
    old_id TEXT PRIMARY KEY,
    new_id INTEGER NOT NULL UNIQUE
);
INSERT INTO serial_activity_ids(old_id,new_id)
SELECT id,ROW_NUMBER() OVER (ORDER BY id) FROM activity_events;

UPDATE activity_events
SET detail_json=json_set(detail_json,'$.play_ids',json((
    SELECT json_group_array(CAST(mapped.new_id AS TEXT))
    FROM (
        SELECT serial_play_ids.new_id
        FROM json_each(activity_events.detail_json,'$.play_ids') AS detail_play
        JOIN serial_play_ids ON serial_play_ids.old_id=detail_play.value
        ORDER BY CAST(detail_play.key AS INTEGER)
    ) AS mapped
)))
WHERE kind='bulk_watch' AND detail_json IS NOT NULL AND json_valid(detail_json);

CREATE TABLE plays_serial (
    id INTEGER PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_id TEXT REFERENCES media(id) ON DELETE CASCADE,
    episode_id TEXT REFERENCES episodes(id) ON DELETE CASCADE,
    watched_at TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'web',
    created_at TEXT NOT NULL,
    CHECK ((media_id IS NOT NULL AND episode_id IS NULL) OR
           (media_id IS NULL AND episode_id IS NOT NULL))
);
INSERT INTO plays_serial(id,user_id,media_id,episode_id,watched_at,source,created_at)
SELECT serial_play_ids.new_id,plays.user_id,plays.media_id,plays.episode_id,plays.watched_at,plays.source,plays.created_at
FROM plays JOIN serial_play_ids ON serial_play_ids.old_id=plays.id;

CREATE TABLE activity_events_serial (
    id INTEGER PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('watch', 'rewatch', 'rating', 'bulk_watch')),
    play_id INTEGER REFERENCES plays_serial(id) ON DELETE CASCADE,
    media_id TEXT REFERENCES media(id) ON DELETE CASCADE,
    episode_id TEXT REFERENCES episodes(id) ON DELETE CASCADE,
    rating INTEGER CHECK (rating IS NULL OR rating BETWEEN 1 AND 5),
    detail_json TEXT,
    occurred_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);
INSERT INTO activity_events_serial(id,user_id,kind,play_id,media_id,episode_id,rating,detail_json,occurred_at,created_at)
SELECT serial_activity_ids.new_id,activity_events.user_id,activity_events.kind,serial_play_ids.new_id,
       activity_events.media_id,activity_events.episode_id,activity_events.rating,activity_events.detail_json,
       activity_events.occurred_at,activity_events.created_at
FROM activity_events
JOIN serial_activity_ids ON serial_activity_ids.old_id=activity_events.id
LEFT JOIN serial_play_ids ON serial_play_ids.old_id=activity_events.play_id;

DROP TABLE activity_events;
DROP TABLE plays;
ALTER TABLE plays_serial RENAME TO plays;
ALTER TABLE activity_events_serial RENAME TO activity_events;
CREATE INDEX idx_plays_user_watched ON plays(user_id, watched_at DESC);
CREATE INDEX idx_plays_user_episode ON plays(user_id, episode_id);
CREATE INDEX idx_activity_events_feed ON activity_events(occurred_at DESC, id);
CREATE INDEX idx_activity_events_created_at ON activity_events(created_at);

DROP TABLE serial_user_ids;
DROP TABLE serial_users;
DROP TABLE serial_play_ids;
DROP TABLE serial_activity_ids;
