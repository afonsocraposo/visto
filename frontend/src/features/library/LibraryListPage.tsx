import { InfiniteScrollTrigger } from "../../components/InfiniteScrollTrigger";
import { useInfiniteQuery } from "@tanstack/react-query";
import { Alert, Button, Group, Loader, SegmentedControl, Select, Text, Title } from "@mantine/core";
import { IconArrowLeft } from "@tabler/icons-react";
import { useState } from "react";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import type { CursorPage, LibraryEntry, LibraryStatus, MediaDetailTarget } from "../../types";
import { pageURL } from "../../lib/pagination";
import { LibraryCard } from "./LibraryCard";
import { librarySortOptions, type LibrarySort } from "./librarySort";
import { libraryMediaFilterOptions, type LibraryMediaFilter } from "./mediaFilter";

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
  onMediaFilterChange,
  onBack,
  onOpenDetail,
}: {
  status: LibraryStatus;
  mediaFilter: LibraryMediaFilter;
  onMediaFilterChange: (mediaFilter: LibraryMediaFilter) => void;
  onBack: () => void;
  onOpenDetail?: (target: MediaDetailTarget) => void;
}) {
  const [sort, setSort] = useState<LibrarySort>("updated");
  const userQueryKey = useUserQueryKey();
  const library = useInfiniteQuery({
    queryKey: [...userQueryKey("library"), sort, status, mediaFilter],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      api.get<CursorPage<LibraryEntry>>(
        pageURL(
          `/api/v1/library?sort=${sort}&status=${status}${mediaFilter === "all" ? "" : `&media_type=${mediaFilter}`}`,
          pageParam,
        ),
        "Your library is temporarily unavailable.",
      ),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
  if (library.isPending)
    return (
      <Group justify="center" mt="xl">
        <Loader />
      </Group>
    );
  if (library.isError)
    return (
      <Alert color="red" mt="md">
        Your library is temporarily unavailable.
      </Alert>
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
          {entries.length} {entries.length === 1 ? "title" : "titles"} loaded.
        </Text>
      </div>
      <Group mb="xl" align="end" justify="space-between">
        <SegmentedControl
          aria-label="Filter library by media type"
          value={mediaFilter}
          onChange={(value) => onMediaFilterChange(value as LibraryMediaFilter)}
          data={libraryMediaFilterOptions}
        />
        <Select
          label="Sort by"
          aria-label="Sort library media"
          data={librarySortOptions}
          value={sort}
          onChange={(value) => value && setSort(value as LibrarySort)}
          allowDeselect={false}
        />
      </Group>
      {entries.length === 0 ? (
        <Text c="dimmed">This list is empty.</Text>
      ) : (
        <div className="poster-grid">
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
