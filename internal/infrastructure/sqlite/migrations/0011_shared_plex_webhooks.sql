CREATE TABLE plex_webhooks_new (
    id INTEGER PRIMARY KEY,
    user_id INTEGER UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    account_id TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    last_used_at TEXT
);
INSERT INTO plex_webhooks_new(user_id,token_hash,account_id,created_at,last_used_at)
SELECT user_id,token_hash,account_id,created_at,last_used_at FROM plex_webhooks;
DROP TABLE plex_webhooks;
ALTER TABLE plex_webhooks_new RENAME TO plex_webhooks;
CREATE UNIQUE INDEX idx_plex_webhooks_shared ON plex_webhooks((1)) WHERE user_id IS NULL;

ALTER TABLE users ADD COLUMN plex_account_id TEXT
    CHECK (plex_account_id IS NULL OR (length(plex_account_id) BETWEEN 1 AND 20 AND plex_account_id NOT GLOB '*[^0-9]*'));
CREATE UNIQUE INDEX idx_users_plex_account_id ON users(plex_account_id) WHERE plex_account_id IS NOT NULL;

CREATE TABLE plex_observed_accounts (
    account_id TEXT PRIMARY KEY CHECK (length(account_id) BETWEEN 1 AND 20 AND account_id NOT GLOB '*[^0-9]*'),
    title TEXT NOT NULL DEFAULT '',
    last_seen_at TEXT NOT NULL
);
CREATE INDEX idx_plex_observed_accounts_recent ON plex_observed_accounts(last_seen_at DESC);
