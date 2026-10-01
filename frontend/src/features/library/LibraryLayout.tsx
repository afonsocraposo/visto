import { Button, Group, SegmentedControl, Select, Text, TextInput, Title } from "@mantine/core";
import { IconArrowsSort, IconChevronRight, IconSearch } from "@tabler/icons-react";
import type { ReactNode } from "react";
import { RouteLink } from "../../components/RouteLink";
import type { LibraryStatus } from "../../types";
import { librarySortOptions, type LibrarySort } from "./librarySort";
import { libraryMediaFilterOptions, type LibraryMediaFilter } from "./mediaFilter";

/**
 * Presentation shared by your own Library and other people's shared libraries.
 * Data fetching stays with each page because the two APIs return different shapes.
 */

export const libraryStatusLabels: Record<LibraryStatus, string> = {
  watching: "Watching",
  watchlist: "Watchlist",
  paused: "Paused",
  completed: "Completed",
  dropped: "Dropped",
};

export const librarySections: Array<{ status: LibraryStatus; label: string }> = (
  ["watching", "watchlist", "paused", "completed", "dropped"] as const
).map((status) => ({ status, label: libraryStatusLabels[status] }));

/** Shown as full poster rows; the others are compact links so the page stays short. */
export const primaryLibraryStatuses: LibraryStatus[] = ["watching", "watchlist", "completed"];
export const LIBRARY_PREVIEW_LIMIT = 6;

export function titleCount(count: number): string {
  return `${count} ${count === 1 ? "title" : "titles"}`;
}

export function LibraryControls({
  mediaFilter,
  onMediaFilterChange,
  sort,
  onSortChange,
}: {
  mediaFilter: LibraryMediaFilter;
  onMediaFilterChange: (mediaFilter: LibraryMediaFilter) => void;
  sort: LibrarySort;
  onSortChange: (sort: LibrarySort) => void;
}) {
  return (
    <Group className="library-controls" justify="space-between" align="center" wrap="nowrap">
      <SegmentedControl
        aria-label="Filter library by media type"
        value={mediaFilter}
        onChange={(value) => onMediaFilterChange(value as LibraryMediaFilter)}
        data={libraryMediaFilterOptions}
      />
      <Select
        className="library-sort"
        aria-label="Sort library media"
        leftSection={<IconArrowsSort size={16} />}
        data={librarySortOptions}
        value={sort}
        onChange={(value) => {
          if (value) onSortChange(value as LibrarySort);
        }}
        allowDeselect={false}
        comboboxProps={{ position: "bottom-end", width: 200 }}
      />
    </Group>
  );
}

export function LibrarySearchInput({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <TextInput
      label={label}
      placeholder="Search titles"
      value={value}
      onChange={(event) => onChange(event.currentTarget.value)}
      leftSection={<IconSearch size={18} />}
      mb="md"
    />
  );
}

export function LibrarySectionHeading({
  status,
  count,
  showAllHref,
  onShowAll,
  order = 2,
}: {
  status: LibraryStatus;
  count: number;
  /** Nested under another "Library" heading on profile pages; looks the same either way. */
  order?: 2 | 3;
  /** Omitted when the preview already shows everything. */
  showAllHref?: string;
  onShowAll?: () => void;
}) {
  return (
    <Group
      className="library-section-heading"
      justify="space-between"
      align="baseline"
      gap="sm"
      wrap="nowrap"
    >
      <Group gap={8} align="baseline" wrap="nowrap">
        <Title id={`library-section-${status}`} order={order} size="h2">
          {libraryStatusLabels[status]}
        </Title>
        <Text size="sm" c="dimmed">
          {titleCount(count)}
        </Text>
      </Group>
      {showAllHref && (
        <Button
          component={RouteLink}
          href={showAllHref}
          variant="subtle"
          size="compact-sm"
          rightSection={<IconChevronRight size={15} />}
          onOpen={onShowAll}
        >
          Show all
        </Button>
      )}
    </Group>
  );
}

export function LibrarySection({
  status,
  heading,
  children,
}: {
  status: LibraryStatus;
  heading: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="library-section" aria-labelledby={`library-section-${status}`}>
      {heading}
      {children}
    </section>
  );
}

/** Compact links for the secondary statuses; statuses with no titles are left out. */
export function LibraryMoreLists({
  lists,
  hrefFor,
  onOpen,
}: {
  lists: Array<{ status: LibraryStatus; count: number }>;
  hrefFor: (status: LibraryStatus) => string;
  onOpen?: (status: LibraryStatus) => void;
}) {
  const visible = lists.filter((list) => list.count > 0);
  if (!visible.length) return null;
  return (
    <nav className="library-more-lists" aria-label="More lists">
      {visible.map((list) => (
        <RouteLink
          key={list.status}
          href={hrefFor(list.status)}
          className="library-list-link"
          onOpen={onOpen ? () => onOpen(list.status) : undefined}
        >
          <span>{libraryStatusLabels[list.status]}</span>
          <span className="library-list-link-count">{list.count}</span>
          <IconChevronRight size={16} aria-hidden="true" />
        </RouteLink>
      ))}
    </nav>
  );
}

export function LibraryListHeading({
  kicker,
  status,
  count,
}: {
  kicker: string;
  status: LibraryStatus;
  count: number;
}) {
  return (
    <div className="page-heading">
      <Text className="section-kicker">{kicker}</Text>
      <Title order={1}>{libraryStatusLabels[status]}</Title>
      <Text c="dimmed" mt={6}>
        {titleCount(count)}
      </Text>
    </div>
  );
}
