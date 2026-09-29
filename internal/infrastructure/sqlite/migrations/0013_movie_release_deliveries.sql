CREATE TABLE movie_release_deliveries (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_id TEXT NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    channel TEXT NOT NULL CHECK (channel IN ('pushover', 'web_push')),
    subscription_id TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL CHECK (state IN ('pending', 'sending', 'sent', 'failed')),
    attempted_at TEXT,
    sent_at TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TEXT,
    PRIMARY KEY (user_id, media_id, channel, subscription_id)
);
CREATE INDEX idx_movie_release_deliveries_state ON movie_release_deliveries(state, next_attempt_at);

CREATE TRIGGER remove_movie_release_web_deliveries_after_push_removal
AFTER DELETE ON web_push_subscriptions
BEGIN
    DELETE FROM movie_release_deliveries
    WHERE user_id=OLD.user_id AND channel='web_push' AND subscription_id=OLD.id;
END;
