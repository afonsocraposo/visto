import type { LibraryEntry, MediaDetailTarget } from "../../types";
import { MediaPosterCard } from "../../components/MediaPosterCard";

export function LibraryCard({ entry, onOpenDetail }: { entry: LibraryEntry; onOpenDetail?: (target: MediaDetailTarget) => void }) {
  const target = { mediaType: entry.media.type, tmdbID: entry.media.tmdb_id, mediaID: entry.item.media_id } as MediaDetailTarget;
  const progress = entry.media.type === "tv" && !entry.completed && entry.progress && entry.progress.total_episodes > 0
    ? { value: Math.min(100, Math.round(entry.progress.watched_episodes / entry.progress.total_episodes * 100)), label: `${entry.media.title} watched progress` }
    : undefined;
  return <MediaPosterCard media={entry.media} target={target} onOpenDetail={onOpenDetail} progress={progress} />;
}
