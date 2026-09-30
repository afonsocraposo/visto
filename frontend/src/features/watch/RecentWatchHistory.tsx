import { useQuery } from "@tanstack/react-query";
import { Alert, Image, Paper, Text } from "@mantine/core";
import { ActivityTime } from "../../components/ActivityTime";
import { ListSkeleton } from "../../components/ListSkeleton";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import { posterURL } from "../../lib/artwork";
import { episodePosition } from "../../lib/episodePosition";
import type { CursorPage, HistoryEntry, MediaDetailTarget } from "../../types";

/** Mounted only once the user reveals it, so the request is made on demand. */
export function RecentWatchHistory({
  onOpenDetail,
}: {
  onOpenDetail?: (target: MediaDetailTarget) => void;
}) {
  const userQueryKey = useUserQueryKey();
  const history = useQuery({
    queryKey: userQueryKey("history", "recent"),
    queryFn: () =>
      api.get<CursorPage<HistoryEntry>>(
        "/api/v1/plays?limit=10",
        "Watch history is temporarily unavailable.",
      ),
  });

  if (history.isPending)
    return (
      <div className="watch-history-list" aria-busy="true">
        <ListSkeleton
          count={3}
          rowClassName="watch-history-row"
          artClassName="watch-history-row-art"
          contentClassName="watch-history-row-content"
          lines={3}
          padded={false}
        />
      </div>
    );
  if (history.isError)
    return (
      <Alert color="red" variant="light">
        Watch history is temporarily unavailable.
      </Alert>
    );

  const entries = history.data.items;
  if (entries.length === 0)
    return (
      <Text size="sm" c="dimmed">
        Movies and episodes you watch will appear here.
      </Text>
    );
  return (
    <div className="watch-history-list">
      {entries.map((entry) => (
        <RecentWatchCard key={entry.play.id} entry={entry} onOpenDetail={onOpenDetail} />
      ))}
    </div>
  );
}

function RecentWatchCard({
  entry,
  onOpenDetail,
}: {
  entry: HistoryEntry;
  onOpenDetail?: (target: MediaDetailTarget) => void;
}) {
  const isEpisode = !!entry.play.episode_id;
  const art = posterURL(entry.artwork_path, "w185");
  const open = () => {
    if (!entry.tmdb_id) return;
    onOpenDetail?.(
      isEpisode
        ? {
            mediaType: "tv",
            tmdbID: entry.tmdb_id,
            episodeID: entry.play.episode_id!,
            ...episodePosition(entry.episode_label),
          }
        : {
            mediaType: "movie",
            tmdbID: entry.tmdb_id,
            mediaID: entry.play.media_id ?? undefined,
          },
    );
  };
  const subtitle = isEpisode
    ? [entry.episode_label, entry.episode_name].filter(Boolean).join(" · ")
    : "Movie";
  return (
    <Paper
      className="watch-history-row"
      withBorder
      p={0}
      role="button"
      tabIndex={0}
      onClick={open}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          open();
        }
      }}
    >
      <div className="watch-history-row-art">
        {art ? (
          <Image src={art} alt="" />
        ) : (
          <div className="artwork-fallback">{entry.title.slice(0, 1)}</div>
        )}
      </div>
      <div className="watch-history-row-content">
        <Text fw={700} size="sm" lineClamp={1}>
          {entry.title}
        </Text>
        <Text size="xs" c="dimmed" lineClamp={1}>
          {subtitle}
        </Text>
        <Text size="xs" c="dimmed">
          <ActivityTime value={entry.play.watched_at} />
        </Text>
      </div>
    </Paper>
  );
}
