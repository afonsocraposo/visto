import { useQuery } from "@tanstack/react-query";
import { Alert, Group, Loader, Text, Title } from "@mantine/core";
import { EmptyState } from "../../components/EmptyState";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import { LibraryCard } from "./LibraryCard";
import type { LibraryEntry, MediaDetailTarget } from "../../types";
export { HistoryPanel } from "./HistoryPanel";
export { ProfilePanel } from "./ProfilePanel";

export function LibraryPanel({ onOpenDetail }: { onOpenDetail?: (target: MediaDetailTarget) => void }) {
  const userQueryKey = useUserQueryKey();
  const library = useQuery({
    queryKey: userQueryKey("library"),
    queryFn: () => api.get<LibraryEntry[]>("/api/v1/library", "Your library is temporarily unavailable."),
  });
  if (library.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (library.isError) return <Alert color="red" mt="md">Your library is temporarily unavailable.</Alert>;
  if (!library.data?.length) return <EmptyState title="Your library is empty" detail="Search for something you want to watch, then make this space your own." />;

  return (
    <>
      <div className="page-heading"><Text className="section-kicker">Your collection</Text><Title order={1}>Library</Title></div>
      {library.data.map(entry => <LibraryCard key={entry.item.media_id} entry={entry} onOpenDetail={onOpenDetail} />)}
    </>
  );
}
