import { useQuery } from "@tanstack/react-query";
import { Alert, Button, Group, Loader, Text, Title } from "@mantine/core";
import { IconArrowLeft } from "@tabler/icons-react";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import type { LibraryEntry, LibraryStatus, MediaDetailTarget } from "../../types";
import { LibraryCard } from "./LibraryCard";

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
  const userQueryKey = useUserQueryKey();
  const library = useQuery({
    queryKey: userQueryKey("library"),
    queryFn: () =>
      api.get<LibraryEntry[]>("/api/v1/library", "Your library is temporarily unavailable."),
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
  const entries = library.data
    .filter((entry) => (status === "completed" ? entry.completed : entry.item.status === status))
    .sort((a, b) => Date.parse(b.item.updated_at) - Date.parse(a.item.updated_at));
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
          {entries.length} {entries.length === 1 ? "title" : "titles"}, sorted by recent updates.
        </Text>
      </div>
      {entries.length === 0 ? (
        <Text c="dimmed">This list is empty.</Text>
      ) : (
        <div className="library-grid library-grid-full">
          {entries.map((entry) => (
            <LibraryCard key={entry.item.media_id} entry={entry} onOpenDetail={onOpenDetail} />
          ))}
        </div>
      )}
    </div>
  );
}
