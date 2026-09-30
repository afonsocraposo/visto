ALTER TABLE plex_webhook_events ADD COLUMN event_type TEXT;
ALTER TABLE plex_webhook_events ADD COLUMN raw_payload TEXT;
