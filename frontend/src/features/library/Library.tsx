import { useQueries, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import {
  Alert,
  Button,
  Group,
  Loader,
  Paper,
  SegmentedControl,
  Select,
  Text,
  Title,
} from "@mantine/core";
import { IconSearch } from "@tabler/icons-react";
import { useState } from "react";
import { EmptyState } from "../../components/EmptyState";
import { ImportData } from "./ImportData";
import { useSessionUserID, useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import { LibraryCard } from "./LibraryCard";
import type { CursorPage, LibraryEntry, LibraryStatus, MediaDetailTarget } from "../../types";
import { librarySortOptions, type LibrarySort } from "./librarySort";
import { libraryMediaFilterOptions, type LibraryMediaFilter } from "./mediaFilter";
import {
  readLibraryFilter,
  readLibrarySort,
  saveLibraryFilter,
  saveLibrarySort,
} from "./libraryPreferences";
export { HistoryPanel } from "./HistoryPanel";
export { ProfilePanel } from "./ProfilePanel";

const sections: Array<{ status: LibraryStatus; label: string }> = [
  { status: "watching", label: "Watching" },
  { status: "watchlist", label: "Watchlist" },
  { status: "paused", label: "Paused" },
  { status: "completed", label: "Completed" },
  { status: "dropped", label: "Dropped" },
];
const PREVIEW_LIMIT = 6;

export function LibraryPanel({
  onOpenDetail,
  onOpenList,
}: {
  onOpenDetail?: (target: MediaDetailTarget) => void;
  onOpenList?: (status: LibraryStatus, mediaFilter: LibraryMediaFilter) => void;
}) {
  const navigate = useNavigate();
  const userID = useSessionUserID();
  const [mediaFilter, setMediaFilter] = useState<LibraryMediaFilter>(() =>
    readLibraryFilter(userID),
  );
  const [sort, setSort] = useState<LibrarySort>(() => readLibrarySort(userID));
  const userQueryKey = useUserQueryKey();
  const libraries = useQueries({
    queries: sections.map((section) => ({
      queryKey: [...userQueryKey("library"), "preview", sort, section.status, mediaFilter],
      queryFn: () =>
        api.get<CursorPage<LibraryEntry>>(
          `/api/v1/library?sort=${sort}&status=${section.status}&limit=${PREVIEW_LIMIT}${mediaFilter === "all" ? "" : `&media_type=${mediaFilter}`}`,
          "Your library is temporarily unavailable.",
        ),
      refetchOnMount: "always" as const,
      refetchOnWindowFocus: "always" as const,
    })),
  });
  const anyLibrary = useQuery({
    queryKey: [...userQueryKey("library"), "any"],
    queryFn: () =>
      api.get<CursorPage<LibraryEntry>>(
        "/api/v1/library?limit=1",
        "Your library is temporarily unavailable.",
      ),
    refetchOnMount: "always",
    refetchOnWindowFocus: "always",
  });
  if (libraries.some((library) => library.isPending) || anyLibrary.isPending)
    return (
      <Group justify="center" mt="xl">
        <Loader />
      </Group>
    );
  if (libraries.some((library) => library.isError) || anyLibrary.isError)
    return (
      <Alert color="red" mt="md">
        Your library is temporarily unavailable.
      </Alert>
    );
  if (libraries.every((library) => !library.data?.items.length) && !anyLibrary.data?.items.length)
    return (
      <>
        <div className="page-heading">
          <Text className="section-kicker">Your collection</Text>
          <Title order={1}>Library</Title>
        </div>
        <Paper className="library-empty" withBorder radius="lg">
          <Title order={2}>Start with your watch history</Title>
          <Text c="dimmed" mt="sm">
            Bring your shows, movies, and ratings from Bingers, or find something new to watch.
          </Text>
          <Group className="library-empty-actions" mt="xl">
            <ImportData prominent refreshLibraryOnClose />
            <Button
              variant="subtle"
              size="lg"
              leftSection={<IconSearch size={18} />}
              onClick={() => void navigate({ to: "/discover" })}
            >
              Browse trending
            </Button>
          </Group>
        </Paper>
      </>
    );
  return (
    <>
      <div className="page-heading">
        <Text className="section-kicker">Your collection</Text>
        <Title order={1}>Library</Title>
        <Text c="dimmed" mt={6}>
          A quick view of everything you are tracking.
        </Text>
      </div>
      <Group mb="xl" align="end" justify="space-between">
        <SegmentedControl
          aria-label="Filter library by media type"
          value={mediaFilter}
          onChange={(value) => {
            const next = value as LibraryMediaFilter;
            setMediaFilter(next);
            saveLibraryFilter(userID, next);
          }}
          data={libraryMediaFilterOptions}
        />
        <Select
          label="Sort by"
          aria-label="Sort library media"
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
      {libraries.every((library) => !library.data?.items.length) && (
        <EmptyState
          title="No titles in this filter"
          detail="Choose another media type to see your library."
        />
      )}
      <div className="library-sections">
        {sections.map((section, index) => {
          const page = libraries[index].data!;
          const entries = page.items;
          if (entries.length === 0) return null;
          const totalCount = page.total_count ?? entries.length;
          return (
            <section
              className="library-section"
              key={section.status}
              aria-labelledby={`library-section-${section.status}`}
            >
              <Group
                className="library-section-heading"
                justify="space-between"
                align="baseline"
                gap="sm"
              >
                <div>
                  <Title id={`library-section-${section.status}`} order={2}>
                    {section.label}
                  </Title>
                  <Text size="sm" c="dimmed">
                    {totalCount} {totalCount === 1 ? "title" : "titles"}
                  </Text>
                </div>
                {page.next_cursor && (
                  <Button
                    variant="subtle"
                    size="sm"
                    onClick={() => onOpenList?.(section.status, mediaFilter)}
                  >
                    Show all
                  </Button>
                )}
              </Group>
              <div className="poster-grid">
                {entries.map((entry) => (
                  <LibraryCard
                    key={entry.item.media_id}
                    entry={entry}
                    onOpenDetail={onOpenDetail}
                  />
                ))}
              </div>
            </section>
          );
        })}
      </div>
    </>
  );
}

export { sections as librarySections, PREVIEW_LIMIT };
