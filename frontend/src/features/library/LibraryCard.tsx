import { Image, Paper, Text } from "@mantine/core";
import type { LibraryEntry, MediaDetailTarget } from "../../types";
import { posterURL } from "../../lib/artwork";

export function LibraryCard({ entry, onOpenDetail }: { entry: LibraryEntry; onOpenDetail?: (target: MediaDetailTarget) => void }) {
  const art = posterURL(entry.media.poster_path, "w342");
  const target = { mediaType: entry.media.type, tmdbID: entry.media.tmdb_id, mediaID: entry.item.media_id } as MediaDetailTarget;
  return <Paper className="library-tile" component="button" type="button" onClick={() => onOpenDetail?.(target)} aria-label={`Open ${entry.media.title}`}>
    {art ? <Image src={art} alt="" /> : <div className="artwork-fallback">{entry.media.title.slice(0, 1)}</div>}
    <span className="library-tile-scrim" aria-hidden="true" />
    <Text className="library-tile-title" component="span" fw={700}>{entry.media.title}</Text>
  </Paper>;
}
