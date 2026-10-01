import { keepPreviousData, useQueries, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Button, Group, Paper, Skeleton, Text, Title } from "@mantine/core";
import { IconSearch } from "@tabler/icons-react";
import { QueryError } from "../../components/QueryError";
import { useState, type ReactNode } from "react";
import { EmptyState } from "../../components/EmptyState";
import { PosterGridSkeleton } from "../../components/PosterGridSkeleton";
import { ImportData } from "./ImportData";
import { RouteLink } from "../../components/RouteLink";
import { useSessionUserID, useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import { LibraryCard } from "./LibraryCard";
import type { CursorPage, LibraryEntry, LibraryStatus, MediaDetailTarget } from "../../types";
import type { LibrarySort } from "./librarySort";
import type { LibraryMediaFilter } from "./mediaFilter";
import {
  readLibraryFilter,
  readLibrarySort,
  saveLibraryFilter,
  saveLibrarySort,
} from "./libraryPreferences";
import {
  LIBRARY_PREVIEW_LIMIT,
  LibraryControls,
  LibraryMoreLists,
  LibrarySection,
  LibrarySectionHeading,
  librarySections as sections,
  primaryLibraryStatuses,
} from "./LibraryLayout";
export { HistoryPanel } from "./HistoryPanel";
export { ProfilePanel } from "./ProfilePanel";

export function LibraryPanel({
  onOpenDetail,
  onOpenList,
  headerAction,
}: {
  onOpenDetail?: (target: MediaDetailTarget) => void;
  onOpenList?: (status: LibraryStatus, mediaFilter: LibraryMediaFilter) => void;
  headerAction?: ReactNode;
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
          `/api/v1/library?sort=${sort}&status=${section.status}&limit=${LIBRARY_PREVIEW_LIMIT}${mediaFilter === "all" ? "" : `&media_type=${mediaFilter}`}`,
          "Your library is temporarily unavailable.",
        ),
      // Keep the current posters while a new filter/sort loads; the grid just dims briefly.
      placeholderData: keepPreviousData,
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
  const heading = (
    <Group className="page-heading" justify="space-between" align="center" wrap="nowrap">
      <Title order={1}>Library</Title>
      {headerAction}
    </Group>
  );
  if (libraries.some((library) => library.isPending) || anyLibrary.isPending)
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
  if (libraries.some((library) => library.isError) || anyLibrary.isError)
    return (
      <>
        {heading}
        <QueryError
          message="Could not load your library."
          onRetry={() =>
            Promise.all([anyLibrary.refetch(), ...libraries.map((library) => library.refetch())])
          }
        />
      </>
    );
  if (libraries.every((library) => !library.data?.items.length) && !anyLibrary.data?.items.length)
    return (
      <>
        {heading}
        <Paper className="library-empty" withBorder radius="lg">
          <Title order={2}>Start with your watch history</Title>
          <Text c="dimmed" mt="sm">
            Bring your shows, movies, and ratings from Bingers, or find something new to watch.
          </Text>
          <Group className="library-empty-actions" mt="xl">
            <ImportData prominent refreshLibraryOnClose />
            <Button
              component={RouteLink}
              href="/discover"
              variant="subtle"
              size="lg"
              leftSection={<IconSearch size={18} />}
              onOpen={() => void navigate({ to: "/discover" })}
            >
              Browse trending
            </Button>
          </Group>
        </Paper>
      </>
    );
  const refreshing = libraries.some((library) => library.isPlaceholderData) || undefined;
  const pages = new Map(sections.map((section, index) => [section.status, libraries[index].data!]));
  const countOf = (status: LibraryStatus) => {
    const page = pages.get(status)!;
    return page.total_count ?? page.items.length;
  };
  const listHref = (status: LibraryStatus) =>
    `/profile/library/${status}${mediaFilter === "all" ? "" : `?media_type=${mediaFilter}`}`;
  return (
    <>
      {heading}
      <LibraryControls
        mediaFilter={mediaFilter}
        onMediaFilterChange={(next) => {
          setMediaFilter(next);
          saveLibraryFilter(userID, next);
        }}
        sort={sort}
        onSortChange={(next) => {
          setSort(next);
          saveLibrarySort(userID, next);
        }}
      />
      {libraries.every((library) => !library.data?.items.length) && (
        <EmptyState
          title={`No ${mediaFilter === "movie" ? "movies" : "TV shows"} yet`}
          detail="Choose another media type to see the rest of your library."
          action={
            <Button
              variant="light"
              onClick={() => {
                setMediaFilter("all");
                saveLibraryFilter(userID, "all");
              }}
            >
              Show everything
            </Button>
          }
        />
      )}
      <div className="library-sections content-ready">
        {sections
          .filter((section) => primaryLibraryStatuses.includes(section.status))
          .map((section) => {
            const page = pages.get(section.status)!;
            const entries = page.items;
            return (
              <LibrarySection
                key={section.status}
                status={section.status}
                heading={
                  <LibrarySectionHeading
                    status={section.status}
                    count={countOf(section.status)}
                    showAllHref={page.next_cursor ? listHref(section.status) : undefined}
                    onShowAll={() => onOpenList?.(section.status, mediaFilter)}
                  />
                }
              >
                {entries.length ? (
                  <div className="poster-grid" data-refreshing={refreshing}>
                    {entries.map((entry, index) => (
                      <LibraryCard
                        key={entry.item.media_id}
                        entry={entry}
                        eager={index < 6 && section.status === "watching"}
                        onOpenDetail={onOpenDetail}
                      />
                    ))}
                  </div>
                ) : section.status === "completed" ? (
                  <Text size="sm" c="dimmed" className="library-section-empty">
                    Finished shows and watched movies will appear here.
                  </Text>
                ) : (
                  <EmptyState
                    title={
                      section.status === "watchlist"
                        ? "Your watchlist is empty"
                        : "You're not watching anything yet"
                    }
                    detail={
                      section.status === "watchlist"
                        ? "Save things you're interested in from Discover."
                        : "Add a show and its next episode will appear in Watching."
                    }
                    action={
                      <Button
                        component={RouteLink}
                        href="/discover"
                        variant="light"
                        onOpen={() => void navigate({ to: "/discover" })}
                      >
                        {section.status === "watchlist" ? "Browse trending" : "Discover shows"}
                      </Button>
                    }
                  />
                )}
              </LibrarySection>
            );
          })}
      </div>
      <LibraryMoreLists
        lists={sections
          .filter((section) => !primaryLibraryStatuses.includes(section.status))
          .map((section) => ({ status: section.status, count: countOf(section.status) }))}
        hrefFor={listHref}
        onOpen={(status) => onOpenList?.(status, mediaFilter)}
      />
    </>
  );
}
