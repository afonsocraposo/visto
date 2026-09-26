CREATE INDEX idx_user_media_paging_updated ON user_media(user_id, updated_at DESC, media_id);
CREATE INDEX idx_plays_user_media_watched ON plays(user_id, media_id, watched_at DESC, id DESC);
CREATE INDEX idx_plays_user_episode_watched ON plays(user_id, episode_id, watched_at DESC, id DESC);
CREATE INDEX idx_episodes_season_paging ON episodes(season_id, season_number, episode_number, id);
CREATE INDEX idx_users_created_paging ON users(created_at, id);
