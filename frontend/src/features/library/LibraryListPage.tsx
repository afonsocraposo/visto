import { useQuery } from "@tanstack/react-query";
import { Alert, Button, Group, Loader, Select, Text, Title } from "@mantine/core";
import { IconArrowLeft } from "@tabler/icons-react";
import { useState } from "react";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import type { LibraryEntry, LibraryStatus, MediaDetailTarget } from "../../types";
import { LibraryCard } from "./LibraryCard";
import { librarySortOptions, type LibrarySort } from "./librarySort";

const labels: Record<LibraryStatus, string> = {
  watching: "Watching",
  completed: "Completed",
  watchlist: "Watchlist",
  paused: "Paused",
  dropped: "Dropped",
};

export function LibraryListPage({
  status,
  onBack,
  onOpenDetail,
}: {
  status: LibraryStatus;
  onBack: () => void;
  onOpenDetail?: (target: MediaDetailTarget) => void;
}) {
  const [sort, setSort] = useState<LibrarySort>("updated");
  const userQueryKey = useUserQueryKey();
  const library = useQuery({
    queryKey: [...userQueryKey("library"), sort],
    queryFn: () =>
      api.get<LibraryEntry[]>(
        `/api/v1/library?sort=${sort}`,
        "Your library is temporarily unavailable.",
      ),
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
  const entries = library.data.filter((entry) =>
    status === "completed" ? entry.completed : !entry.completed && entry.item.status === status,
  );
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
          {entries.length} {entries.length === 1 ? "title" : "titles"}.
        </Text>
      </div>
      <Select
        label="Sort by"
        aria-label="Sort library media"
        data={librarySortOptions}
        value={sort}
        onChange={(value) => value && setSort(value as LibrarySort)}
        allowDeselect={false}
        mb="xl"
      />
      {entries.length === 0 ? (
        <Text c="dimmed">This list is empty.</Text>
      ) : (
        <div className="poster-grid">
          {entries.map((entry) => (
            <LibraryCard key={entry.item.media_id} entry={entry} onOpenDetail={onOpenDetail} />
          ))}
        </div>
      )}
    </div>
  );
}
