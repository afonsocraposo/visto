import { useQuery } from "@tanstack/react-query";
import { IconCheck } from "@tabler/icons-react";
import { ActivityTime } from "../../components/ActivityTime";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import { backdropURL, posterURL } from "../../lib/artwork";
import { episodeLabelCode, episodePosition } from "../../lib/episodePosition";
import type { CursorPage, HistoryEntry, MediaDetailTarget } from "../../types";
import { WatchRowCard } from "./WatchRowCard";

/** The 10 latest plays, fetched ahead of time so revealing them is instant. */
export function useRecentWatches() {
  const userQueryKey = useUserQueryKey();
  return useQuery({
    queryKey: userQueryKey("history", "recent"),
    queryFn: () =>
      api.get<CursorPage<HistoryEntry>>(
        "/api/v1/plays?limit=10",
        "Watch history is temporarily unavailable.",
      ),
    staleTime: 5 * 60_000,
    select: (page) => page.items,
  });
}

/** Oldest first, so the newest play sits right above the episodes still to watch. */
export function RecentWatchRows({
  entries,
  onOpenDetail,
}: {
  entries: HistoryEntry[];
  onOpenDetail?: (target: MediaDetailTarget) => void;
}) {
  return (
    <>
      {[...entries].reverse().map((entry) => (
        <RecentWatchRow key={entry.play.id} entry={entry} onOpenDetail={onOpenDetail} />
      ))}
    </>
  );
}

function RecentWatchRow({
  entry,
  onOpenDetail,
}: {
  entry: HistoryEntry;
  onOpenDetail?: (target: MediaDetailTarget) => void;
}) {
  const isEpisode = !!entry.play.episode_id;
  const show = entry.tmdb_id
    ? () =>
        onOpenDetail?.({
          mediaType: isEpisode ? "tv" : "movie",
          tmdbID: entry.tmdb_id!,
          mediaID: entry.play.media_id ?? undefined,
        })
    : undefined;
  const open = entry.tmdb_id
    ? isEpisode
      ? () =>
          onOpenDetail?.({
            mediaType: "tv",
            tmdbID: entry.tmdb_id!,
            episodeID: entry.play.episode_id!,
            ...episodePosition(entry.episode_label),
          })
      : show
    : undefined;
  // Episodes carry a still (or the show poster as fallback), movies a poster.
  const art = isEpisode
    ? backdropURL(entry.artwork_path, "w780")
    : posterURL(entry.artwork_path, "w500");
  return (
    <WatchRowCard
      className="watch-row-watched"
      show={isEpisode ? entry.title : "Movie"}
      onOpenShow={onOpenDetail && isEpisode ? show : undefined}
      title={isEpisode ? entry.episode_name || "Episode" : entry.title}
      openLabel={`Open ${entry.title}${isEpisode && entry.episode_label ? ` ${entry.episode_label}` : ""}`}
      meta={
        <>
          {isEpisode && entry.episode_label && <span>{episodeLabelCode(entry.episode_label)}</span>}
          <span>
            <ActivityTime value={entry.play.watched_at} />
          </span>
        </>
      }
      art={art}
      onOpen={onOpenDetail ? open : undefined}
      trailing={
        <span className="watch-row-watched-mark" aria-label="Watched">
          <IconCheck size={18} stroke={2.4} aria-hidden="true" />
        </span>
      }
    />
  );
}
