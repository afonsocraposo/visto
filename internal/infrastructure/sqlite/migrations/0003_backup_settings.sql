CREATE TABLE backup_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    destination TEXT NOT NULL,
    interval_seconds INTEGER NOT NULL,
    s3_bucket TEXT NOT NULL DEFAULT '',
    s3_region TEXT NOT NULL DEFAULT '',
    s3_endpoint TEXT NOT NULL DEFAULT '',
    s3_access_key_id TEXT NOT NULL DEFAULT '',
    s3_secret_ciphertext TEXT NOT NULL DEFAULT '',
    s3_path_style INTEGER NOT NULL DEFAULT 0,
    s3_prefix TEXT NOT NULL DEFAULT 'visto/',
    s3_max_keep INTEGER NOT NULL DEFAULT 30,
    last_success_at TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT ''
);
