CREATE TABLE season_ready_alerts (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    season_id TEXT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    ready_at TEXT,
    PRIMARY KEY (user_id, season_id)
);

CREATE TABLE season_ready_deliveries (
    user_id INTEGER NOT NULL,
    season_id TEXT NOT NULL,
    channel TEXT NOT NULL CHECK (channel IN ('pushover', 'web_push')),
    subscription_id TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL CHECK (state IN ('pending', 'sending', 'sent', 'failed')),
    attempted_at TEXT,
    sent_at TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TEXT,
    PRIMARY KEY (user_id, season_id, channel, subscription_id),
    FOREIGN KEY (user_id, season_id) REFERENCES season_ready_alerts(user_id, season_id) ON DELETE CASCADE
);
CREATE INDEX idx_season_ready_deliveries_state ON season_ready_deliveries(state, next_attempt_at);

CREATE TRIGGER cancel_season_alerts_after_library_status_change
AFTER UPDATE OF status ON user_media WHEN OLD.status='watching' AND NEW.status!='watching'
BEGIN
    DELETE FROM season_ready_alerts WHERE user_id=NEW.user_id AND season_id IN
        (SELECT id FROM seasons WHERE show_id=NEW.media_id);
END;

CREATE TRIGGER cancel_season_alerts_after_library_removal
AFTER DELETE ON user_media WHEN OLD.status='watching'
BEGIN
    DELETE FROM season_ready_alerts WHERE user_id=OLD.user_id AND season_id IN
        (SELECT id FROM seasons WHERE show_id=OLD.media_id);
END;

CREATE TRIGGER remove_season_alert_deliveries_after_push_removal
AFTER DELETE ON web_push_subscriptions
BEGIN
    DELETE FROM season_ready_deliveries
    WHERE user_id=OLD.user_id AND channel='web_push' AND subscription_id=OLD.id;
END;
