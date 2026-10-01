-- Plex sends the episode's TMDB ID; this lets a watched episode be found in the local catalog
-- (and with it the show) without asking TMDB.
CREATE INDEX idx_episodes_tmdb_id ON episodes(tmdb_id) WHERE tmdb_id IS NOT NULL;

-- The history names the show and the episode ("S2E4") instead of the episode title.
ALTER TABLE plex_webhook_events ADD COLUMN episode_label TEXT;

UPDATE plex_webhook_events
SET episode_label = 'S' || json_extract(raw_payload, '$.Metadata.parentIndex') || 'E' || json_extract(raw_payload, '$.Metadata.index'),
    title = CASE
        WHEN status = 'synced' THEN title
        ELSE COALESCE(NULLIF(trim(json_extract(raw_payload, '$.Metadata.grandparentTitle')), ''), title)
    END
WHERE raw_payload IS NOT NULL
  AND json_valid(raw_payload)
  AND json_extract(raw_payload, '$.Metadata.type') = 'episode'
  AND json_type(raw_payload, '$.Metadata.parentIndex') = 'integer'
  AND json_type(raw_payload, '$.Metadata.index') = 'integer'
  AND json_extract(raw_payload, '$.Metadata.parentIndex') >= 0
  AND json_extract(raw_payload, '$.Metadata.index') > 0;
