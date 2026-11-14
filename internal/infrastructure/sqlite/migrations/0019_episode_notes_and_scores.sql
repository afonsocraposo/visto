CREATE TABLE episode_notes (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    episode_id TEXT NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    note TEXT NOT NULL CHECK(length(note) <= 5000),
    updated_at TEXT NOT NULL,
    PRIMARY KEY(user_id, episode_id)
);

-- A series and its episodes share a TMDB show ID but have separate scores.
CREATE TABLE external_ratings_next (
    media_type TEXT NOT NULL CHECK(media_type IN ('movie','tv')),
    tmdb_id INTEGER NOT NULL,
    season_number INTEGER NOT NULL DEFAULT 0 CHECK(season_number >= 0),
    episode_number INTEGER NOT NULL DEFAULT 0 CHECK(episode_number >= 0),
    data TEXT NOT NULL,
    fetched_at TEXT NOT NULL,
    PRIMARY KEY(media_type, tmdb_id, season_number, episode_number)
);
INSERT INTO external_ratings_next(media_type,tmdb_id,data,fetched_at)
SELECT media_type,tmdb_id,data,fetched_at FROM external_ratings;
DROP TABLE external_ratings;
ALTER TABLE external_ratings_next RENAME TO external_ratings;
