import { useQuery } from "@tanstack/react-query";
import { Badge, Text } from "@mantine/core";
import { IconCheck } from "@tabler/icons-react";
import { ActivityTime } from "../../components/ActivityTime";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import { backdropURL, posterURL } from "../../lib/artwork";
import { episodePosition } from "../../lib/episodePosition";
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
  const position = entry.episode_label?.replace("E", " | E");
  return (
    <WatchRowCard
      className="watch-row-watched"
      title={entry.title}
      art={art}
      onOpen={onOpenDetail ? open : undefined}
      trailing={
        <div className="watch-row-watched-meta">
          <IconCheck size={18} stroke={2.2} aria-hidden="true" />
          <Text size="xs" c="dimmed">
            <ActivityTime value={entry.play.watched_at} />
          </Text>
        </div>
      }
    >
      <Badge
        component="button"
        type="button"
        className="watch-row-show"
        size="lg"
        variant="outline"
        color="gray"
        radius="xl"
        aria-label={`Open ${entry.title}`}
        disabled={!onOpenDetail || !show}
        onClick={(event) => {
          event.stopPropagation();
          show?.();
        }}
      >
        {entry.title}
      </Badge>
      <Text className="watch-row-episode">{isEpisode ? (position ?? "Episode") : "Movie"}</Text>
      {isEpisode && entry.episode_name && (
        <Text className="watch-row-name" lineClamp={1}>
          {entry.episode_name}
        </Text>
      )}
    </WatchRowCard>
  );
}
