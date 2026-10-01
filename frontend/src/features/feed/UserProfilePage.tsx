import { keepPreviousData, useInfiniteQuery, useQueries, useQuery } from "@tanstack/react-query";
import { QueryError } from "../../components/QueryError";
import { Alert, Avatar, Button, Group, Skeleton, Text, Title } from "@mantine/core";
import { IconArrowLeft } from "@tabler/icons-react";
import { useState } from "react";
import { EmptyState } from "../../components/EmptyState";
import { SectionTabs } from "../../components/SectionTabs";
import { InfiniteScrollTrigger } from "../../components/InfiniteScrollTrigger";
import { PosterGridSkeleton } from "../../components/PosterGridSkeleton";
import { ListSkeleton } from "../../components/ListSkeleton";
import { useUserQueryKey } from "../auth/SessionContext";
import { api, APIRequestError } from "../../lib/api";
import { pageURL } from "../../lib/pagination";
import type { LibrarySort } from "../library/librarySort";
import type { LibraryMediaFilter } from "../library/mediaFilter";
import {
  readCommunityLibraryFilter,
  readCommunityLibrarySort,
  saveCommunityLibraryFilter,
  saveCommunityLibrarySort,
} from "../library/libraryPreferences";
import {
  LIBRARY_PREVIEW_LIMIT,
  LibraryControls,
  LibraryMoreLists,
  LibrarySection,
  LibrarySectionHeading,
  librarySections,
  primaryLibraryStatuses,
} from "../library/LibraryLayout";
import type {
  CommunityLibraryItem,
  CommunityProfile,
  CursorPage,
  FeedItem,
  LibraryStatus,
  MediaDetailTarget,
} from "../../types";
import { CommunityLibraryCard } from "./CommunityLibraryCard";
import { communityLibraryURL } from "./communityLibrary";
import { ActivityRow } from "./ActivityRow";
import { activityAction, episodeLabel } from "./FeedPanel";

export function UserProfilePage({
  userID,
  onBack,
  onOpenDetail,
  listHref,
  onOpenList,
}: {
  userID: string;
  onBack: () => void;
  onOpenDetail?: (target: MediaDetailTarget) => void;
  listHref: (status: LibraryStatus, mediaFilter: LibraryMediaFilter) => string;
  onOpenList: (status: LibraryStatus, mediaFilter: LibraryMediaFilter) => void;
}) {
  const userQueryKey = useUserQueryKey();
  const [section, setSection] = useState<"library" | "activity">("library");
  const base = `/api/v1/community/users/${encodeURIComponent(userID)}`;
  const profile = useQuery({
    queryKey: userQueryKey("community-profile", userID),
    queryFn: () => api.get<CommunityProfile>(base, "Profile is temporarily unavailable."),
    refetchOnMount: "always",
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
                  <CommunityLibraryOverview
                    key={userID}
                    userID={userID}
                    listHref={listHref}
                    onOpenList={onOpenList}
                    onOpenDetail={onOpenDetail}
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
                          {items.map((item) => {
                            const showTarget: MediaDetailTarget | undefined =
                              item.media_type && item.tmdb_id
                                ? { mediaType: item.media_type, tmdbID: item.tmdb_id }
                                : undefined;
                            return (
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
                                detailTarget={
                                  showTarget
                                    ? {
                                        ...showTarget,
                                        seasonNumber: item.season_number,
                                        episodeNumber: item.episode_number,
                                      }
                                    : undefined
                                }
                                showTarget={showTarget}
                                onOpenDetail={onOpenDetail}
                              />
                            );
                          })}
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

/** Mirrors your own Library: one small preview query per status, never the whole collection. */
function CommunityLibraryOverview({
  userID,
  listHref,
  onOpenList,
  onOpenDetail,
}: {
  userID: string;
  listHref: (status: LibraryStatus, mediaFilter: LibraryMediaFilter) => string;
  onOpenList: (status: LibraryStatus, mediaFilter: LibraryMediaFilter) => void;
  onOpenDetail?: (target: MediaDetailTarget) => void;
}) {
  const userQueryKey = useUserQueryKey();
  const [mediaFilter, setMediaFilter] = useState<LibraryMediaFilter>(() =>
    readCommunityLibraryFilter(userID),
  );
  const [sort, setSort] = useState<LibrarySort>(() => readCommunityLibrarySort(userID));
  const libraries = useQueries({
    queries: librarySections.map((section) => ({
      queryKey: userQueryKey(
        "community-library",
        userID,
        "preview",
        sort,
        section.status,
        mediaFilter,
      ),
      queryFn: () =>
        api.get<CursorPage<CommunityLibraryItem>>(
          communityLibraryURL(userID, {
            status: section.status,
            sort,
            mediaFilter,
            limit: LIBRARY_PREVIEW_LIMIT,
          }),
          "Library is temporarily unavailable.",
        ),
      placeholderData: keepPreviousData,
    })),
  });
  const heading = (
    <Title order={2} mb="md">
      Library
    </Title>
  );
  if (libraries.some((library) => library.isPending))
    return (
      <>
        {heading}
        <Group className="library-controls" justify="space-between">
          <Skeleton height={36} width={200} radius="md" />
          <Skeleton height={36} width={150} radius="md" />
        </Group>
        <div className="library-sections">
          {Array.from({ length: 2 }).map((_, i) => (
            <section className="library-section" key={i}>
              <Skeleton height={22} width={140} mb="sm" />
              <PosterGridSkeleton count={6} />
            </section>
          ))}
        </div>
      </>
    );
  if (libraries.some((library) => library.isError))
    return (
      <>
        {heading}
        <QueryError
          message="Could not load this library."
          onRetry={() => Promise.all(libraries.map((library) => library.refetch()))}
        />
      </>
    );
  const refreshing = libraries.some((library) => library.isPlaceholderData) || undefined;
  const pages = new Map(
    librarySections.map((section, index) => [section.status, libraries[index].data!]),
  );
  const countOf = (status: LibraryStatus) => {
    const page = pages.get(status)!;
    return page.total_count ?? page.items.length;
  };
  const changeFilter = (next: LibraryMediaFilter) => {
    setMediaFilter(next);
    saveCommunityLibraryFilter(userID, next);
  };
  const empty = librarySections.every((section) => countOf(section.status) === 0);
  // Unlike your own Library there is nothing to act on in an empty section, so skip it.
  const primary = librarySections.filter(
    (section) => primaryLibraryStatuses.includes(section.status) && countOf(section.status) > 0,
  );
  return (
    <>
      {heading}
      <LibraryControls
        mediaFilter={mediaFilter}
        onMediaFilterChange={changeFilter}
        sort={sort}
        onSortChange={(next) => {
          setSort(next);
          saveCommunityLibrarySort(userID, next);
        }}
      />
      {empty ? (
        <EmptyState
          title={mediaFilter === "all" ? "This library is empty" : "No titles in this filter"}
          detail={
            mediaFilter === "all"
              ? "This user hasn't added anything yet."
              : "Choose another media type to see the rest of this library."
          }
          action={
            mediaFilter === "all" ? undefined : (
              <Button variant="light" onClick={() => changeFilter("all")}>
                Show everything
              </Button>
            )
          }
        />
      ) : (
        <>
          {primary.length > 0 && (
            <div className="library-sections content-ready">
              {primary.map((section) => {
                const page = pages.get(section.status)!;
                return (
                  <LibrarySection
                    key={section.status}
                    status={section.status}
                    heading={
                      <LibrarySectionHeading
                        status={section.status}
                        count={countOf(section.status)}
                        order={3}
                        showAllHref={
                          page.next_cursor ? listHref(section.status, mediaFilter) : undefined
                        }
                        onShowAll={() => onOpenList(section.status, mediaFilter)}
                      />
                    }
                  >
                    <div className="poster-grid" data-refreshing={refreshing}>
                      {page.items.map((entry) => (
                        <CommunityLibraryCard
                          key={entry.media.id}
                          entry={entry}
                          onOpenDetail={onOpenDetail}
                        />
                      ))}
                    </div>
                  </LibrarySection>
                );
              })}
            </div>
          )}
          <LibraryMoreLists
            lists={librarySections
              .filter((section) => !primaryLibraryStatuses.includes(section.status))
              .map((section) => ({ status: section.status, count: countOf(section.status) }))}
            hrefFor={(status) => listHref(status, mediaFilter)}
            onOpen={(status) => onOpenList(status, mediaFilter)}
          />
        </>
      )}
    </>
  );
}
