CREATE TABLE web_push_subscriptions (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    encrypted_subscription TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_web_push_subscriptions_user ON web_push_subscriptions(user_id);

CREATE TABLE web_push_deliveries (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    episode_id TEXT NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
    subscription_id TEXT NOT NULL REFERENCES web_push_subscriptions(id) ON DELETE CASCADE,
    state TEXT NOT NULL CHECK (state IN ('sending', 'sent', 'failed')),
    attempted_at TEXT NOT NULL,
    sent_at TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 1 CHECK (attempt_count > 0),
    next_attempt_at TEXT,
    PRIMARY KEY (user_id, episode_id, subscription_id)
);
CREATE INDEX idx_web_push_deliveries_state ON web_push_deliveries(state, attempted_at);
