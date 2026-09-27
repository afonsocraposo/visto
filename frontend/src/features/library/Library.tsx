import { useQueries } from "@tanstack/react-query";
import { Alert, Button, Group, Loader, SegmentedControl, Select, Text, Title } from "@mantine/core";
import { useState } from "react";
import { EmptyState } from "../../components/EmptyState";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import { LibraryCard } from "./LibraryCard";
import type { CursorPage, LibraryEntry, LibraryStatus, MediaDetailTarget } from "../../types";
import { librarySortOptions, type LibrarySort } from "./librarySort";
import { libraryMediaFilterOptions, type LibraryMediaFilter } from "./mediaFilter";
export { HistoryPanel } from "./HistoryPanel";
export { ProfilePanel } from "./ProfilePanel";

const sections: Array<{ status: LibraryStatus; label: string }> = [
  { status: "watching", label: "Watching" },
  { status: "completed", label: "Completed" },
  { status: "watchlist", label: "Watchlist" },
  { status: "paused", label: "Paused" },
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
  const [mediaFilter, setMediaFilter] = useState<LibraryMediaFilter>("all");
  const [sort, setSort] = useState<LibrarySort>("updated");
  const userQueryKey = useUserQueryKey();
  const libraries = useQueries({
    queries: sections.map((section) => ({
      queryKey: [...userQueryKey("library"), sort, section.status, mediaFilter],
      queryFn: () =>
        api.get<CursorPage<LibraryEntry>>(
          `/api/v1/library?sort=${sort}&status=${section.status}&limit=${PREVIEW_LIMIT}${mediaFilter === "all" ? "" : `&media_type=${mediaFilter}`}`,
          "Your library is temporarily unavailable.",
        ),
    })),
  });
  if (libraries.some((library) => library.isPending))
    return (
      <Group justify="center" mt="xl">
        <Loader />
      </Group>
    );
  if (libraries.some((library) => library.isError))
    return (
      <Alert color="red" mt="md">
        Your library is temporarily unavailable.
      </Alert>
    );
  if (libraries.every((library) => !library.data?.items.length))
    return (
      <EmptyState
        title="Your library is empty"
        detail="Search for something you want to watch, then make this space your own."
      />
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
          onChange={(value) => setMediaFilter(value as LibraryMediaFilter)}
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
      <div className="library-sections">
        {sections.map((section, index) => {
          const page = libraries[index].data!;
          const entries = page.items;
          if (entries.length === 0) return null;
          const visible = entries;
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
                    {entries.length} {entries.length === 1 ? "title" : "titles"} shown
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
                {visible.map((entry) => (
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
