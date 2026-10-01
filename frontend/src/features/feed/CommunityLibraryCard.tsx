import { MediaPosterCard } from "../../components/MediaPosterCard";
import type { CommunityLibraryItem, MediaDetailTarget } from "../../types";

export function CommunityLibraryCard({
  entry,
  onOpenDetail,
  eager,
}: {
  entry: CommunityLibraryItem;
  onOpenDetail?: (target: MediaDetailTarget) => void;
  eager?: boolean;
}) {
  const progress =
    entry.progress && entry.status !== "completed" && entry.progress.total_episodes > 0
      ? {
          value: Math.min(
            100,
            Math.round((entry.progress.watched_episodes / entry.progress.total_episodes) * 100),
          ),
          label: `${entry.media.title} watched progress`,
        }
      : undefined;
  return (
    <MediaPosterCard
      media={entry.media}
      target={{ mediaType: entry.media.type, tmdbID: entry.media.tmdb_id }}
      onOpenDetail={onOpenDetail}
      progress={progress}
      eager={eager}
    />
  );
}
