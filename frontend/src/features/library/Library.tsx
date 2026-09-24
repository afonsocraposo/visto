import { useQuery } from "@tanstack/react-query";
import { Alert, Badge, Group, Loader, Tabs, Text, Title } from "@mantine/core";
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
  const sections = [
    { value: "watching", label: "Watching" },
    { value: "watchlist", label: "Watchlist" },
    { value: "paused", label: "Paused" },
    { value: "dropped", label: "Dropped" },
  ];

  return (
    <>
      <div className="page-heading"><Text className="section-kicker">Your collection</Text><Title order={1}>Library</Title></div>
      <Tabs defaultValue="watching" keepMounted={false}>
        <Tabs.List className="library-status-tabs" grow>
          {sections.map(section => <Tabs.Tab key={section.value} value={section.value} rightSection={<Badge size="sm" variant="light">{library.data.filter(entry => entry.item.status === section.value).length}</Badge>}>{section.label}</Tabs.Tab>)}
        </Tabs.List>
        {sections.map(section => {
          const entries = library.data.filter(entry => entry.item.status === section.value);
          return <Tabs.Panel key={section.value} value={section.value} pt="md">
            {entries.length === 0
              ? <EmptyState title={`No titles in ${section.label.toLowerCase()}`} detail="Move a title here from its detail page when your plans change." />
              : <div className="library-grid">{entries.map(entry => <LibraryCard key={entry.item.media_id} entry={entry} onOpenDetail={onOpenDetail} />)}</div>}
          </Tabs.Panel>;
        })}
      </Tabs>
    </>
  );
}
