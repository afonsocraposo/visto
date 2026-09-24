import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Group, Paper, Select, Text } from "@mantine/core";
import { useUserQueryKey } from "../auth/SessionContext";
import { EpisodeBrowser } from "./EpisodeBrowser";
import type { LibraryEntry } from "../../types";

export function LibraryCard({ entry }: { entry: LibraryEntry }) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const [episodesOpen, setEpisodesOpen] = useState(false);
  const update = useMutation({
    mutationFn: async ({ status, rating }: { status: string; rating: number | null }) => {
      const response = await fetch(`/api/v1/library/${encodeURIComponent(entry.item.media_id)}`, {
        method: "PATCH", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ status, rating }),
      });
      if (!response.ok) throw new Error("Could not update this title.");
    },
    onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("continue") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("calendar") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
    ]),
  });
  const watched = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/plays", {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ media_id: entry.item.media_id }),
      });
      if (!response.ok) throw new Error("Could not record this watch.");
    }, onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("history") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
    ]),
  });

  return <Paper withBorder p="md" mt="sm">
    <Group justify="space-between">
      <Text fw={700}>{entry.media.title}</Text>
      <Text size="sm" c="dimmed">{entry.media.type === "tv" ? "TV show" : "Movie"}</Text>
    </Group>
    <Group mt="sm" grow>
      <Select aria-label={`Status for ${entry.media.title}`} value={entry.item.status} onChange={status => {
        if (status) update.mutate({ status, rating: entry.item.rating });
      }} data={[
        { value: "watchlist", label: "Watchlist" }, { value: "watching", label: "Watching" },
        { value: "paused", label: "Paused" }, { value: "dropped", label: "Dropped" },
      ]} />
      <Select aria-label={`Rating for ${entry.media.title}`} value={entry.item.rating ? String(entry.item.rating) : "none"} onChange={value => {
        update.mutate({ status: entry.item.status, rating: value && value !== "none" ? Number(value) : null });
      }} data={[
        { value: "none", label: "Not rated" },
        ...[1, 2, 3, 4, 5].map(value => ({ value: String(value), label: `${value} / 5 stars` })),
      ]} />
    </Group>
    {entry.media.type === "tv" && <Button mt="sm" size="xs" variant="default" onClick={() => setEpisodesOpen(true)}>Browse episodes</Button>}
    {entry.media.type === "movie" && <Button mt="sm" size="xs" loading={watched.isPending} onClick={() => watched.mutate()}>Watched</Button>}
    {update.isError && <Alert color="red" mt="sm">{update.error.message}</Alert>}
    {watched.isError && <Alert color="red" mt="sm">{watched.error.message}</Alert>}
    {episodesOpen && <EpisodeBrowser showID={entry.item.media_id} title={entry.media.title} onClose={() => setEpisodesOpen(false)} />}
  </Paper>;
}
