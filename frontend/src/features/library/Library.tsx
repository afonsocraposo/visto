import { useQuery } from "@tanstack/react-query";
import { Alert, Group, Loader, Title } from "@mantine/core";
import { EmptyState } from "../../components/EmptyState";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import { LibraryCard } from "./LibraryCard";
import type { LibraryEntry } from "../../types";
export { HistoryPanel } from "./HistoryPanel";
export { ProfilePanel } from "./ProfilePanel";

export function LibraryPanel() {
  const userQueryKey = useUserQueryKey();
  const library = useQuery({
    queryKey: userQueryKey("library"),
    queryFn: () => api.get<LibraryEntry[]>("/api/v1/library", "Your library is temporarily unavailable."),
  });
  if (library.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (library.isError) return <Alert color="red" mt="md">Your library is temporarily unavailable.</Alert>;
  if (!library.data?.length) return <EmptyState title="Your library is empty" />;

  return (
    <>
      <Title order={1}>Library</Title>
      {library.data.map(entry => <LibraryCard key={entry.item.media_id} entry={entry} />)}
    </>
  );
}
