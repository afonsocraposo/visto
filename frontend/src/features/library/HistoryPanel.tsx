import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Group, Loader, Paper, Text, TextInput, Title } from "@mantine/core";
import { useUserQueryKey } from "../auth/SessionContext";
import type { HistoryEntry } from "../../types";

export function HistoryPanel() {
  const userQueryKey = useUserQueryKey();
  const history = useQuery({
    queryKey: userQueryKey("history"),
    queryFn: async () => {
      const response = await fetch("/api/v1/plays?limit=100");
      if (!response.ok) throw new Error();
      return response.json() as Promise<HistoryEntry[]>;
    },
  });
  if (history.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (history.isError) return <Alert color="red" mt="lg">Watch history is temporarily unavailable.</Alert>;

  return <Paper withBorder p="md" mt="lg">
    <Title order={2}>Watch history</Title>
    {history.data.length === 0
      ? <Text c="dimmed" mt="sm">No watches recorded yet.</Text>
      : history.data.map(entry => <HistoryCard key={entry.play.id} entry={entry} />)}
  </Paper>;
}

function HistoryCard({ entry }: { entry: HistoryEntry }) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const [watchedAt, setWatchedAt] = useState(() => {
    const date = new Date(entry.play.watched_at);
    return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
  });
  const refreshHistory = () => Promise.all([
    queryClient.invalidateQueries({ queryKey: userQueryKey("history") }),
    queryClient.invalidateQueries({ queryKey: userQueryKey("continue") }),
    queryClient.invalidateQueries({ queryKey: userQueryKey("calendar") }),
    queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
    queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
  ]);
  const save = useMutation({
    mutationFn: async () => {
      const response = await fetch(`/api/v1/plays/${encodeURIComponent(entry.play.id)}`, {
        method: "PATCH", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ watched_at: new Date(watchedAt).toISOString() }),
      });
      if (!response.ok) throw new Error("Could not correct this watch.");
    }, onSuccess: refreshHistory,
  });
  const remove = useMutation({
    mutationFn: async () => {
      const response = await fetch(`/api/v1/plays/${encodeURIComponent(entry.play.id)}`, { method: "DELETE" });
      if (!response.ok) throw new Error("Could not delete this watch.");
    }, onSuccess: refreshHistory,
  });
  const rewatch = useMutation({
    mutationFn: async () => {
      const body = entry.play.media_id ? { media_id: entry.play.media_id } : { episode_id: entry.play.episode_id };
      const response = await fetch("/api/v1/plays", {
        method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body),
      });
      if (!response.ok) throw new Error("Could not record the rewatch.");
    }, onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("history") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
    ]),
  });

  return <Group justify="space-between" align="end" mt="md" wrap="wrap">
    <div>
      <Text fw={600}>{entry.title}{entry.episode_label ? ` · ${entry.episode_label}` : ""}</Text>
      <Text size="xs" c="dimmed">Watched {new Date(entry.play.watched_at).toLocaleString()}</Text>
    </div>
    <Group align="end" gap="xs">
      <TextInput type="datetime-local" aria-label={`Watched time for ${entry.title}`} value={watchedAt} onChange={event => setWatchedAt(event.currentTarget.value)} w={210} />
      <Button size="xs" variant="default" disabled={!watchedAt} loading={save.isPending} onClick={() => save.mutate()}>Correct</Button>
      <Button size="xs" variant="light" loading={rewatch.isPending} onClick={() => rewatch.mutate()}>Rewatch</Button>
      <Button size="xs" color="red" variant="subtle" loading={remove.isPending} onClick={() => { if (window.confirm("Delete this individual watch?")) remove.mutate(); }}>Delete</Button>
    </Group>
    {(save.isError || rewatch.isError || remove.isError) && <Alert color="red" w="100%">{save.error?.message || rewatch.error?.message || remove.error?.message}</Alert>}
  </Group>;
}
