import { keepPreviousData, useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useDebouncedValue } from "@mantine/hooks";
import { Alert, Button, Group, Skeleton } from "@mantine/core";
import { IconArrowLeft } from "@tabler/icons-react";
import { useState } from "react";
import { EmptyState } from "../../components/EmptyState";
import { InfiniteScrollTrigger } from "../../components/InfiniteScrollTrigger";
import { PosterGridSkeleton } from "../../components/PosterGridSkeleton";
import { QueryError } from "../../components/QueryError";
import { api, APIRequestError } from "../../lib/api";
import { pageURL } from "../../lib/pagination";
import type {
  CommunityLibraryItem,
  CommunityProfile,
  CursorPage,
  LibraryStatus,
  MediaDetailTarget,
} from "../../types";
import { useUserQueryKey } from "../auth/SessionContext";
import type { LibrarySort } from "../library/librarySort";
import type { LibraryMediaFilter } from "../library/mediaFilter";
import {
  readCommunityLibrarySort,
  saveCommunityLibraryFilter,
  saveCommunityLibrarySort,
} from "../library/libraryPreferences";
import {
  LibraryControls,
  LibraryListHeading,
  LibrarySearchInput,
  libraryStatusLabels,
} from "../library/LibraryLayout";
import { CommunityLibraryCard } from "./CommunityLibraryCard";
import { communityLibraryURL, possessive } from "./communityLibrary";

/** One status of someone else's shared library, with the same controls as your own list pages. */
export function CommunityLibraryListPage({
  userID,
  status,
  mediaFilter,
  query,
  onMediaFilterChange,
  onQueryChange,
  onBack,
  onOpenDetail,
}: {
  userID: string;
  status: LibraryStatus;
  mediaFilter: LibraryMediaFilter;
  query: string;
  onMediaFilterChange: (mediaFilter: LibraryMediaFilter) => void;
  onQueryChange: (query: string) => void;
  onBack: () => void;
  onOpenDetail?: (target: MediaDetailTarget) => void;
}) {
  const userQueryKey = useUserQueryKey();
  const [sort, setSort] = useState<LibrarySort>(() => readCommunityLibrarySort(userID));
  const [debouncedQuery] = useDebouncedValue(query.trim(), 350);
  const profile = useQuery({
    queryKey: userQueryKey("community-profile", userID),
    queryFn: () =>
      api.get<CommunityProfile>(
        `/api/v1/community/users/${encodeURIComponent(userID)}`,
        "Profile is temporarily unavailable.",
      ),
  });
  const library = useInfiniteQuery({
    queryKey: userQueryKey(
      "community-library",
      userID,
      "page",
      sort,
      status,
      mediaFilter,
      debouncedQuery,
    ),
    enabled: profile.data?.sharing === true,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      api.get<CursorPage<CommunityLibraryItem>>(
        pageURL(
          communityLibraryURL(userID, { status, sort, mediaFilter, query: debouncedQuery }),
          pageParam,
        ),
        "Library is temporarily unavailable.",
      ),
    placeholderData: keepPreviousData,
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
  const owner = profile.data ? `${possessive(profile.data.name)} library` : "Shared library";
  const backButton = (
    <Button
      className="detail-back"
      variant="subtle"
      leftSection={<IconArrowLeft size={16} />}
      onClick={onBack}
    >
      Back to {profile.data?.sharing ? owner : "profile"}
    </Button>
  );
  const searchInput = (
    <LibrarySearchInput
      label={`Search ${profile.data ? owner : "this library"}`}
      value={query}
      onChange={onQueryChange}
    />
  );
  const isPrivate =
    profile.data?.sharing === false ||
    (library.error instanceof APIRequestError && library.error.status === 403);

  if (profile.isError)
    return (
      <div className="library-list-page">
        {backButton}
        <Alert color="red" mt="md">
          {profile.error instanceof APIRequestError && profile.error.status === 404
            ? "User not found."
            : "Profile is temporarily unavailable."}
        </Alert>
      </div>
    );
  if (isPrivate)
    return (
      <div className="library-list-page">
        {backButton}
        <EmptyState
          title="Private profile"
          detail="This user has not shared their library or activity with this instance."
        />
      </div>
    );
  if (profile.isPending || library.isPending)
    return (
      <div className="library-list-page">
        <Skeleton height={20} width={180} mb="md" />
        <div className="page-heading">
          <Skeleton height={12} width={130} mb={8} />
          <Skeleton height={34} width={200} mb={8} />
          <Skeleton height={16} width={90} />
        </div>
        {searchInput}
        <Group mb="xl" justify="space-between" align="end">
          <Skeleton height={36} width={220} radius="xl" />
          <Skeleton height={36} width={160} />
        </Group>
        <PosterGridSkeleton count={12} />
      </div>
    );
  if (library.isError)
    return (
      <div className="library-list-page">
        {backButton}
        {searchInput}
        <QueryError message="Could not load this list." onRetry={() => library.refetch()} />
      </div>
    );
  const entries = library.data.pages.flatMap((page) => page.items);
  return (
    <div className="library-list-page">
      {backButton}
      <LibraryListHeading
        kicker={owner}
        status={status}
        count={library.data.pages[0].total_count ?? entries.length}
      />
      {searchInput}
      <LibraryControls
        mediaFilter={mediaFilter}
        onMediaFilterChange={(next) => {
          saveCommunityLibraryFilter(userID, next);
          onMediaFilterChange(next);
        }}
        sort={sort}
        onSortChange={(next) => {
          setSort(next);
          saveCommunityLibrarySort(userID, next);
        }}
      />
      {entries.length === 0 ? (
        <EmptyState
          title={debouncedQuery ? "No titles match your search" : "This list is empty"}
          detail={
            debouncedQuery
              ? "Try a different title or clear the search."
              : mediaFilter === "all"
                ? `Nothing in ${libraryStatusLabels[status]} yet.`
                : "Choose another media type to see the rest of this list."
          }
        />
      ) : (
        <div className="poster-grid" data-refreshing={library.isPlaceholderData || undefined}>
          {entries.map((entry) => (
            <CommunityLibraryCard key={entry.media.id} entry={entry} onOpenDetail={onOpenDetail} />
          ))}
        </div>
      )}
      <InfiniteScrollTrigger
        hasNextPage={!!library.hasNextPage}
        isFetchingNextPage={library.isFetchingNextPage}
        isFetchNextPageError={library.isFetchNextPageError}
        fetchNextPage={() => void library.fetchNextPage()}
      />
    </div>
  );
}
