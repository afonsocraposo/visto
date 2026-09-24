import type { LibraryEntry, MediaDetailTarget } from "../../types";
import { MediaPosterCard } from "../../components/MediaPosterCard";

export function LibraryCard({ entry, onOpenDetail }: { entry: LibraryEntry; onOpenDetail?: (target: MediaDetailTarget) => void }) {
  const target = { mediaType: entry.media.type, tmdbID: entry.media.tmdb_id, mediaID: entry.item.media_id } as MediaDetailTarget;
  return <MediaPosterCard media={entry.media} target={target} onOpenDetail={onOpenDetail} />;
}
