import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Group, Loader, Modal, Paper, Text } from "@mantine/core";
import { EmptyState } from "../../components/EmptyState";
import type { CalendarEntry, ContinueEntry } from "../../types";

export function WatchNow() {
  const queryClient = useQueryClient();
  const [confirmation, setConfirmation] = useState<ContinueEntry | null>(null);
  const entries = useQuery({
    queryKey: ["continue"],
    queryFn: async () => {
      const response = await fetch("/api/v1/continue-watching");
      if (!response.ok) throw new Error();
      return response.json() as Promise<ContinueEntry[]>;
    },
  });
  const markWatched = useMutation({
    mutationFn: async ({ episodeIDs, bulk }: { episodeIDs: string[]; bulk: boolean }) => {
      const response = await fetch(bulk ? "/api/v1/plays/bulk" : "/api/v1/plays", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(bulk ? { episode_ids: episodeIDs } : { episode_id: episodeIDs[0] }),
      });
      if (!response.ok) throw new Error("Could not mark episode watched.");
    },
    onSuccess: async () => {
      setConfirmation(null);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["continue"] }),
        queryClient.invalidateQueries({ queryKey: ["calendar"] }),
        queryClient.invalidateQueries({ queryKey: ["feed"] }),
        queryClient.invalidateQueries({ queryKey: ["library"] }),
      ]);
    },
  });

  if (entries.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (entries.isError) return <Alert color="red" mt="md">Watch data is temporarily unavailable.</Alert>;
  if (!entries.data?.length) return <EmptyState title="Nothing to continue yet" />;

  return (
    <>
      <Modal opened={confirmation !== null} onClose={() => setConfirmation(null)} title="Skipped episodes" centered>
        <Text mb="md">You have {confirmation?.missing_prior_episodes?.length} earlier unplayed episodes of {confirmation?.title}. How would you like to continue?</Text>
        <Group justify="flex-end">
          <Button variant="default" onClick={() => confirmation?.next_episode && markWatched.mutate({ episodeIDs: [confirmation.next_episode.id], bulk: false })} loading={markWatched.isPending}>Only this episode</Button>
          <Button onClick={() => confirmation?.next_episode && markWatched.mutate({ episodeIDs: [...(confirmation.missing_prior_episodes || []).map(episode => episode.id), confirmation.next_episode.id], bulk: true })} loading={markWatched.isPending}>Mark all as watched</Button>
        </Group>
      </Modal>
      {entries.data.map(entry => (
        <Paper key={entry.show_id} withBorder p="md" mt="md">
          <Group justify="space-between">
            <div>
              <Text fw={700}>{entry.title}</Text>
              <Text c="dimmed">{entry.kind === "start" ? "Ready to start" : "Continue watching"}{entry.next_episode ? ` · S${String(entry.next_episode.season_number).padStart(2, "0")}E${String(entry.next_episode.episode_number).padStart(2, "0")}` : ""}</Text>
            </div>
            {entry.next_episode && (
              <Button size="xs" loading={markWatched.isPending} onClick={() => (entry.missing_prior_episodes?.length ? setConfirmation(entry) : markWatched.mutate({ episodeIDs: [entry.next_episode!.id], bulk: false }))}>
                Watched
              </Button>
            )}
          </Group>
        </Paper>
      ))}
    </>
  );
}

export function WatchCalendar() {
  const calendar = useQuery({
    queryKey: ["calendar"],
    queryFn: async () => {
      const response = await fetch("/api/v1/calendar");
      if (!response.ok) throw new Error();
      return response.json() as Promise<CalendarEntry[]>;
    },
  });
  if (calendar.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (calendar.isError) return <Alert color="red" mt="md">Calendar is temporarily unavailable.</Alert>;
  if (!calendar.data?.length) return <EmptyState title="No upcoming episodes" />;

  return (
    <>
      {calendar.data.map(item => (
        <Paper key={item.episode.id} withBorder p="md" mt="sm">
          <Text fw={700}>{item.title}</Text>
          <Text>{`S${String(item.episode.season_number).padStart(2, "0")}E${String(item.episode.episode_number).padStart(2, "0")}`}</Text>
          <Text c="dimmed">{item.episode.air_date ? new Date(`${item.episode.air_date}T00:00:00`).toLocaleDateString() : "Date not announced"}</Text>
        </Paper>
      ))}
    </>
  );
}
