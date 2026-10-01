import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { QueryError } from "../../components/QueryError";
import {
  Alert,
  Avatar,
  Button,
  Group,
  SegmentedControl,
  Select,
  Skeleton,
  Text,
  Title,
} from "@mantine/core";
import { IconArrowLeft } from "@tabler/icons-react";
import { useState } from "react";
import { EmptyState } from "../../components/EmptyState";
import { SectionTabs } from "../../components/SectionTabs";
import { InfiniteScrollTrigger } from "../../components/InfiniteScrollTrigger";
import { MediaPosterCard } from "../../components/MediaPosterCard";
import { PosterGridSkeleton } from "../../components/PosterGridSkeleton";
import { ListSkeleton } from "../../components/ListSkeleton";
import { useUserQueryKey } from "../auth/SessionContext";
import { api, APIRequestError } from "../../lib/api";
import { pageURL } from "../../lib/pagination";
import { librarySortOptions, type LibrarySort } from "../library/librarySort";
import { libraryMediaFilterOptions, type LibraryMediaFilter } from "../library/mediaFilter";
import type {
  CommunityLibraryItem,
  CommunityProfile,
  CursorPage,
  FeedItem,
  MediaDetailTarget,
} from "../../types";
import { ActivityRow } from "./ActivityRow";
import { activityAction, episodeLabel } from "./FeedPanel";

const sections = [
  { status: "watching", label: "Watching" },
  { status: "watchlist", label: "Watchlist" },
  { status: "paused", label: "Paused" },
  { status: "completed", label: "Completed" },
  { status: "dropped", label: "Dropped" },
] as const;

export function UserProfilePage({
  userID,
  onBack,
  onOpenDetail,
}: {
  userID: string;
  onBack: () => void;
  onOpenDetail?: (target: MediaDetailTarget) => void;
}) {
  const userQueryKey = useUserQueryKey();
  const [mediaFilter, setMediaFilter] = useState<LibraryMediaFilter>("all");
  const [sort, setSort] = useState<LibrarySort>("updated");
  const [section, setSection] = useState<"library" | "activity">("library");
  const base = `/api/v1/community/users/${encodeURIComponent(userID)}`;
  const profile = useQuery({
    queryKey: userQueryKey("community-profile", userID),
    queryFn: () => api.get<CommunityProfile>(base, "Profile is temporarily unavailable."),
    refetchOnMount: "always",
  });
  const library = useInfiniteQuery({
    queryKey: userQueryKey("community-library", userID, sort, mediaFilter),
    enabled: !profile.isFetching && profile.data?.sharing === true,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      api.get<CursorPage<CommunityLibraryItem>>(
        pageURL(
          `${base}/library?sort=${sort}${mediaFilter === "all" ? "" : `&media_type=${mediaFilter}`}`,
          pageParam,
        ),
        "Library is temporarily unavailable.",
      ),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
  const activity = useInfiniteQuery({
    queryKey: userQueryKey("community-activity", userID),
    enabled: !profile.isFetching && profile.data?.sharing === true,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      api.get<CursorPage<FeedItem>>(
        pageURL(`${base}/activity`, pageParam),
        "Activity is temporarily unavailable.",
      ),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });

  return (
    <section className="community-profile-page">
      <Button
        className="detail-back"
        variant="subtle"
        leftSection={<IconArrowLeft size={16} />}
        onClick={onBack}
      >
        Back to activity
      </Button>
      {(profile.isPending || profile.isFetching) && (
        <>
          <Group className="page-heading" gap="md">
            <Skeleton circle height={56} />
            <div>
              <Skeleton height={12} width={140} mb={6} />
              <Skeleton height={28} width={180} />
            </div>
          </Group>
          <Skeleton height={36} width={260} mt="lg" mb="md" radius="sm" />
          <PosterGridSkeleton count={6} />
        </>
      )}
      {profile.isError && (
        <Alert color="red" mt="md">
          {profile.error instanceof APIRequestError && profile.error.status === 404
            ? "User not found."
            : "Profile is temporarily unavailable."}
        </Alert>
      )}
      {profile.isSuccess && !profile.isFetching && (
        <>
          <Group className="page-heading" gap="md">
            <Avatar size={56}>{profile.data.name.trim().slice(0, 1).toLocaleUpperCase()}</Avatar>
            <div>
              <Text className="section-kicker">Community profile</Text>
              <Title order={1}>{profile.data.name}</Title>
            </div>
          </Group>
          {!profile.data.sharing ? (
            <EmptyState
              title="Private profile"
              detail="This user has not shared their library or activity with this instance."
            />
          ) : (
            <>
              <SectionTabs
                label="Profile sections"
                value={section}
                onChange={setSection}
                options={[
                  { value: "library", label: "Library" },
                  { value: "activity", label: "Recent activity" },
                ]}
              />
              {section === "library" ? (
                <div key="library" className="section-panel">
                  <Title order={2} mb="md">
                    Library
                  </Title>
                  <Group mb="xl" align="end" justify="space-between">
                    <SegmentedControl
                      aria-label="Filter library by media type"
                      value={mediaFilter}
                      onChange={(value) => setMediaFilter(value as LibraryMediaFilter)}
                      data={libraryMediaFilterOptions}
                    />
                    <Select
                      label="Sort by"
                      aria-label="Sort library media"
                      data={librarySortOptions}
                      value={sort}
                      onChange={(value) => {
                        if (!value) return;
                        setSort(value as LibrarySort);
                      }}
                      allowDeselect={false}
                    />
                  </Group>
                  {library.isPending && <PosterGridSkeleton count={12} />}
                  {library.isError && (
                    <QueryError
                      message="Could not load this library."
                      onRetry={() => library.refetch()}
                    />
                  )}
                  {library.isSuccess &&
                    (() => {
                      const entries = library.data.pages.flatMap((page) => page.items);
                      return entries.length === 0 ? (
                        <EmptyState
                          title={
                            mediaFilter === "all"
                              ? "This library is empty."
                              : "No titles in this filter"
                          }
                          detail={
                            mediaFilter === "all"
                              ? "This user hasn't added anything yet."
                              : "Choose another media type to see this library."
                          }
                        />
                      ) : (
                        <div className="library-sections">
                          {sections.map((section) => {
                            const sectionItems = entries.filter(
                              (entry) => entry.status === section.status,
                            );
                            return sectionItems.length === 0 ? null : (
                              <section className="library-section" key={section.status}>
                                <Title order={3} mb="sm">
                                  {section.label}
                                </Title>
                                <div className="poster-grid">
                                  {sectionItems.map((entry) => (
                                    <MediaPosterCard
                                      key={entry.media.id}
                                      media={entry.media}
                                      target={{
                                        mediaType: entry.media.type,
                                        tmdbID: entry.media.tmdb_id,
                                      }}
                                      onOpenDetail={onOpenDetail}
                                      progress={
                                        entry.progress &&
                                        entry.status !== "completed" &&
                                        entry.progress.total_episodes > 0
                                          ? {
                                              value: Math.min(
                                                100,
                                                Math.round(
                                                  (entry.progress.watched_episodes /
                                                    entry.progress.total_episodes) *
                                                    100,
                                                ),
                                              ),
                                              label: `${entry.media.title} watched progress`,
                                            }
                                          : undefined
                                      }
                                    />
                                  ))}
                                </div>
                              </section>
                            );
                          })}
                        </div>
                      );
                    })()}
                  <InfiniteScrollTrigger
                    hasNextPage={!!library.hasNextPage}
                    isFetchingNextPage={library.isFetchingNextPage}
                    isFetchNextPageError={library.isFetchNextPageError}
                    fetchNextPage={() => void library.fetchNextPage()}
                  />
                </div>
              ) : (
                <div key="activity" className="section-panel">
                  <Title order={2} mb="md">
                    Recent activity
                  </Title>
                  {activity.isPending && (
                    <div className="activity-list">
                      <ListSkeleton
                        count={5}
                        rowClassName="activity-row"
                        artClassName="activity-row-art"
                        contentClassName="activity-row-main"
                        lines={2}
                      />
                    </div>
                  )}
                  {activity.isError && (
                    <QueryError
                      message="Could not load this activity."
                      onRetry={() => activity.refetch()}
                    />
                  )}
                  {activity.isSuccess &&
                    (() => {
                      const items = activity.data.pages.flatMap((page) => page.items);
                      return items.length === 0 ? (
                        <Text c="dimmed">No shared activity yet.</Text>
                      ) : (
                        <div className="activity-list">
                          {items.map((item) => (
                            <ActivityRow
                              key={item.id}
                              title={item.title}
                              mediaType={item.media_type}
                              artworkPath={item.artwork_path}
                              actor={item.display_name}
                              action={activityAction(item)}
                              episodeLabel={episodeLabel(item)}
                              episodeName={item.episode_name}
                              rating={item.kind === "rating" ? item.rating : undefined}
                              occurredAt={item.occurred_at}
                              onOpenDetail={
                                item.media_type && item.tmdb_id
                                  ? () =>
                                      onOpenDetail?.({
                                        mediaType: item.media_type!,
                                        tmdbID: item.tmdb_id!,
                                        ...(item.season_number !== undefined
                                          ? { seasonNumber: item.season_number }
                                          : {}),
                                        ...(item.episode_number !== undefined
                                          ? { episodeNumber: item.episode_number }
                                          : {}),
                                      })
                                  : undefined
                              }
                              onOpenShow={
                                item.media_type && item.tmdb_id
                                  ? () =>
                                      onOpenDetail?.({
                                        mediaType: item.media_type!,
                                        tmdbID: item.tmdb_id!,
                                      })
                                  : undefined
                              }
                            />
                          ))}
                        </div>
                      );
                    })()}
                  <InfiniteScrollTrigger
                    hasNextPage={!!activity.hasNextPage}
                    isFetchingNextPage={activity.isFetchingNextPage}
                    isFetchNextPageError={activity.isFetchNextPageError}
                    fetchNextPage={() => void activity.fetchNextPage()}
                  />
                </div>
              )}
            </>
          )}
        </>
      )}
    </section>
  );
}
