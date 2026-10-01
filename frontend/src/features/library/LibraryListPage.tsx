import { InfiniteScrollTrigger } from "../../components/InfiniteScrollTrigger";
import { keepPreviousData, useInfiniteQuery } from "@tanstack/react-query";
import { useDebouncedValue } from "@mantine/hooks";
import {
  Alert,
  Button,
  Group,
  SegmentedControl,
  Select,
  Skeleton,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { IconArrowLeft, IconArrowsSort, IconSearch } from "@tabler/icons-react";
import { QueryError } from "../../components/QueryError";
import { EmptyState } from "../../components/EmptyState";
import { useState } from "react";
import { useSessionUserID, useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import type { CursorPage, LibraryEntry, LibraryStatus, MediaDetailTarget } from "../../types";
import { pageURL } from "../../lib/pagination";
import { LibraryCard } from "./LibraryCard";
import { PosterGridSkeleton } from "../../components/PosterGridSkeleton";
import { librarySortOptions, type LibrarySort } from "./librarySort";
import { libraryMediaFilterOptions, type LibraryMediaFilter } from "./mediaFilter";
import { readLibrarySort, saveLibraryFilter, saveLibrarySort } from "./libraryPreferences";

const labels: Record<LibraryStatus, string> = {
  watching: "Watching",
  completed: "Completed",
  watchlist: "Watchlist",
  paused: "Paused",
  dropped: "Dropped",
};

export function LibraryListPage({
  status,
  mediaFilter,
  query,
  onMediaFilterChange,
  onQueryChange,
  onBack,
  onOpenDetail,
}: {
  status: LibraryStatus;
  mediaFilter: LibraryMediaFilter;
  query: string;
  onMediaFilterChange: (mediaFilter: LibraryMediaFilter) => void;
  onQueryChange: (query: string) => void;
  onBack: () => void;
  onOpenDetail?: (target: MediaDetailTarget) => void;
}) {
  const userID = useSessionUserID();
  const [sort, setSort] = useState<LibrarySort>(() => readLibrarySort(userID));
  const [debouncedQuery] = useDebouncedValue(query.trim(), 350);
  const userQueryKey = useUserQueryKey();
  const library = useInfiniteQuery({
    queryKey: [...userQueryKey("library"), "page", sort, status, mediaFilter, debouncedQuery],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      api.get<CursorPage<LibraryEntry>>(
        pageURL(
          `/api/v1/library?sort=${sort}&status=${status}${mediaFilter === "all" ? "" : `&media_type=${mediaFilter}`}${debouncedQuery ? `&q=${encodeURIComponent(debouncedQuery)}` : ""}`,
          pageParam,
        ),
        "Your library is temporarily unavailable.",
      ),
    placeholderData: keepPreviousData,
    refetchOnMount: "always",
    refetchOnWindowFocus: "always",
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
  const searchInput = (
    <TextInput
      label="Search your library"
      placeholder="Search titles"
      value={query}
      onChange={(event) => onQueryChange(event.currentTarget.value)}
      leftSection={<IconSearch size={18} />}
      mb="md"
    />
  );
  if (library.isPending)
    return (
      <div className="library-list-page">
        <Skeleton height={20} width={140} mb="md" />
        <div className="page-heading">
          <Skeleton height={12} width={130} mb={8} />
          <Skeleton height={34} width={200} mb={8} />
          <Skeleton height={16} width={90} />
        </div>
        {searchInput}
        <Group mb="xl" justify="space-between" align="end">
          <Skeleton height={36} width={220} radius="xl" />
          <Skeleton height={60} width={160} />
        </Group>
        <PosterGridSkeleton count={12} />
      </div>
    );
  if (library.isError)
    return (
      <div className="library-list-page">
        {searchInput}
        <QueryError message="Could not load this list." onRetry={() => library.refetch()} />
      </div>
    );
  const entries = library.data.pages.flatMap((page) => page.items);
  return (
    <div className="library-list-page">
      <Button
        className="detail-back"
        variant="subtle"
        leftSection={<IconArrowLeft size={16} />}
        onClick={onBack}
      >
        Back to library
      </Button>
      <div className="page-heading">
        <Text className="section-kicker">Your collection</Text>
        <Title order={1}>{labels[status]}</Title>
        <Text c="dimmed" mt={6}>
          {library.data.pages[0].total_count ?? entries.length}{" "}
          {(library.data.pages[0].total_count ?? entries.length) === 1 ? "title" : "titles"}
        </Text>
      </div>
      {searchInput}
      <Group className="library-controls" align="center" justify="space-between" wrap="nowrap">
        <SegmentedControl
          aria-label="Filter library by media type"
          value={mediaFilter}
          onChange={(value) => {
            const next = value as LibraryMediaFilter;
            saveLibraryFilter(userID, next);
            onMediaFilterChange(next);
          }}
          data={libraryMediaFilterOptions}
        />
        <Select
          className="library-sort"
          aria-label="Sort library media"
          leftSection={<IconArrowsSort size={16} />}
          comboboxProps={{ position: "bottom-end", width: 200 }}
          data={librarySortOptions}
          value={sort}
          onChange={(value) => {
            if (!value) return;
            const next = value as LibrarySort;
            setSort(next);
            saveLibrarySort(userID, next);
          }}
          allowDeselect={false}
        />
      </Group>
      {entries.length === 0 ? (
        <EmptyState
          title={debouncedQuery ? "No titles match your search" : "This list is empty"}
          detail={
            debouncedQuery
              ? "Try a different title or clear the search."
              : "Titles you move to this list will appear here."
          }
        />
      ) : (
        <div className="poster-grid" data-refreshing={library.isPlaceholderData || undefined}>
          {entries.map((entry) => (
            <LibraryCard key={entry.item.media_id} entry={entry} onOpenDetail={onOpenDetail} />
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
