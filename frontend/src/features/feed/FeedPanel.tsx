import { InfiniteScrollTrigger } from "../../components/InfiniteScrollTrigger";
import { QueryError } from "../../components/QueryError";
import { useInfiniteQuery } from "@tanstack/react-query";
import { Alert, Button } from "@mantine/core";
import { EmptyState } from "../../components/EmptyState";
import { ListSkeleton } from "../../components/ListSkeleton";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import type { FeedItem, MediaDetailTarget } from "../../types";
import { ActivityRow } from "./ActivityRow";

export function FeedPanel({
  onOpenDetail,
  onOpenUser,
}: {
  onOpenDetail?: (target: MediaDetailTarget) => void;
  onOpenUser?: (userID: string) => void;
}) {
  const userQueryKey = useUserQueryKey();
  const feed = useInfiniteQuery({
    queryKey: userQueryKey("feed"),
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam }) => {
      const query = pageParam ? `?cursor=${encodeURIComponent(pageParam)}` : "";
      return api.get<{ items: FeedItem[]; next_cursor: string | null }>(
        `/api/v1/feed${query}`,
        "The feed is temporarily unavailable.",
      );
    },
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
  if (feed.isPending)
    return (
      <div className="activity-list activity-list-feed">
        <ListSkeleton
          count={5}
          rowClassName="activity-row"
          artClassName="activity-row-art"
          contentClassName="activity-row-main"
          lines={2}
        />
      </div>
    );
  if (feed.isError)
    return (
      <QueryError message="Could not load community activity." onRetry={() => feed.refetch()} />
    );
  const items = feed.data.pages.flatMap((page) => page.items);
  if (items.length === 0)
    return (
      <EmptyState
        title="No shared activity yet"
        detail="When someone opts in, their watches and ratings will appear here."
      />
    );

  return (
    <>
      <div className="activity-list activity-list-feed content-ready">
        {items.map((item) => {
          const showTarget: MediaDetailTarget | undefined =
            item.media_type && item.tmdb_id
              ? { mediaType: item.media_type, tmdbID: item.tmdb_id }
              : undefined;
          const detailTarget: MediaDetailTarget | undefined = showTarget
            ? {
                ...showTarget,
                seasonNumber: item.season_number,
                episodeNumber: item.episode_number,
              }
            : undefined;
          return (
            <ActivityRow
              key={item.id}
              title={item.title}
              mediaType={item.media_type}
              artworkPath={item.artwork_path}
              actor={item.display_name}
              actorID={item.user_id}
              onOpenActor={() => onOpenUser?.(item.user_id)}
              action={activityAction(item)}
              episodeLabel={episodeLabel(item)}
              episodeName={item.episode_name}
              rating={item.kind === "rating" ? item.rating : undefined}
              occurredAt={item.occurred_at}
              detailTarget={detailTarget}
              showTarget={showTarget}
              onOpenDetail={onOpenDetail}
            />
          );
        })}
      </div>
      <InfiniteScrollTrigger
        hasNextPage={!!feed.hasNextPage}
        isFetchingNextPage={feed.isFetchingNextPage}
        isFetchNextPageError={feed.isFetchNextPageError}
        fetchNextPage={() => void feed.fetchNextPage()}
      />
    </>
  );
}

export function activityAction(item: FeedItem) {
  if (item.kind === "rewatch") return "rewatched";
  if (item.kind === "rating") return "rated";
  if (item.kind === "bulk_watch") {
    const count = item.count ?? 0;
    return `marked ${count} ${count === 1 ? "episode" : "episodes"} of`;
  }
  return "watched";
}

export function episodeLabel(item: FeedItem) {
  if (item.season_number == null || item.episode_number == null) return undefined;
  return `S${String(item.season_number).padStart(2, "0")}E${String(item.episode_number).padStart(2, "0")}`;
}
