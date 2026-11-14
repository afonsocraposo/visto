ALTER TABLE user_media ADD COLUMN note TEXT NOT NULL DEFAULT '';

CREATE TABLE external_ratings (
    media_type TEXT NOT NULL CHECK(media_type IN ('movie','tv')),
    tmdb_id INTEGER NOT NULL,
    data TEXT NOT NULL,
    fetched_at TEXT NOT NULL,
    PRIMARY KEY(media_type, tmdb_id)
);
