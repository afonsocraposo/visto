UPDATE user_media AS um
SET status='completed', updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')
WHERE um.status='watching'
  AND EXISTS (SELECT 1 FROM media m WHERE m.id=um.media_id AND m.media_type='tv'
    AND m.status IN ('Ended','Canceled','Cancelled'))
  AND EXISTS (SELECT 1 FROM episodes e JOIN plays p ON p.episode_id=e.id
    WHERE e.show_id=um.media_id AND p.user_id=um.user_id AND p.source='import')
  AND EXISTS (SELECT 1 FROM episodes e WHERE e.show_id=um.media_id AND e.season_number>0)
  AND (SELECT COUNT(*) FROM episodes e WHERE e.show_id=um.media_id AND e.season_number>0) >=
      COALESCE((SELECT SUM(s.episode_count) FROM seasons s WHERE s.show_id=um.media_id AND s.season_number>0),0)
  AND NOT EXISTS (SELECT 1 FROM episodes e WHERE e.show_id=um.media_id AND e.season_number>0
    AND NOT EXISTS (SELECT 1 FROM plays p WHERE p.user_id=um.user_id AND p.episode_id=e.id));
