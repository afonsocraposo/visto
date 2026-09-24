import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Badge, Button, Group, Loader, Paper, Stack, Text, TextInput, Title } from "@mantine/core";
import { IconClock, IconEdit, IconHistory, IconRefresh, IconTrash } from "@tabler/icons-react";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import type { HistoryEntry } from "../../types";

export function HistoryPanel() {
  const userQueryKey = useUserQueryKey();
  const history = useQuery({
    queryKey: userQueryKey("history"),
    queryFn: () => api.get<HistoryEntry[]>("/api/v1/plays?limit=100", "Watch history is temporarily unavailable."),
  });
  if (history.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (history.isError) return <Alert color="red" mt="lg">Watch history is temporarily unavailable.</Alert>;

  return <div className="history-page">
    <div className="page-heading"><Text className="section-kicker">Your activity</Text><Title order={1}>Watch history</Title><Text className="history-intro" c="dimmed">A record of everything you have watched.</Text></div>
    {history.data.length === 0
      ? <Paper className="history-empty" withBorder p="xl"><IconHistory size={24} /><Text fw={650} mt="sm">No watches recorded yet.</Text><Text size="sm" c="dimmed" mt={4}>Watched episodes and movies will appear here.</Text></Paper>
      : <div className="history-list">{history.data.map(entry => <HistoryCard key={entry.play.id} entry={entry} />)}</div>}
  </div>;
}

function HistoryCard({ entry }: { entry: HistoryEntry }) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const [editing, setEditing] = useState(false);
  const [watchedAt, setWatchedAt] = useState(() => {
    const date = new Date(entry.play.watched_at);
    return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
  });
  const refreshHistory = () => Promise.all([
    queryClient.invalidateQueries({ queryKey: userQueryKey("history") }),
    queryClient.invalidateQueries({ queryKey: userQueryKey("continue") }),
    queryClient.invalidateQueries({ queryKey: userQueryKey("show-progress") }),
    queryClient.invalidateQueries({ queryKey: userQueryKey("calendar") }),
    queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
    queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
  ]);
  const save = useMutation({
    mutationFn: async () => {
      await api.patch(`/api/v1/plays/${encodeURIComponent(entry.play.id)}`, { watched_at: new Date(watchedAt).toISOString() }, "Could not correct this watch.");
    }, onSuccess: refreshHistory,
  });
  const remove = useMutation({
    mutationFn: async () => {
      await api.delete(`/api/v1/plays/${encodeURIComponent(entry.play.id)}`, "Could not delete this watch.");
    }, onSuccess: refreshHistory,
  });
  const rewatch = useMutation({
    mutationFn: async () => {
      const body = entry.play.media_id ? { media_id: entry.play.media_id } : { episode_id: entry.play.episode_id };
      await api.post("/api/v1/plays", body, "Could not record the rewatch.");
    }, onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("history") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
    ]),
  });

  return <Paper className="history-entry" withBorder p="md">
    <div className="history-entry-marker" aria-hidden="true"><IconHistory size={15} /></div>
    <Stack gap="sm" className="history-entry-content">
      <Group justify="space-between" align="flex-start" gap="md" wrap="nowrap">
        <div className="history-entry-title"><Text fw={750}>{entry.title}</Text>{entry.episode_label && <Badge className="history-episode" variant="light">{entry.episode_label}</Badge>}</div>
        <Text className="history-date" size="sm" c="dimmed">{new Date(entry.play.watched_at).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" })}</Text>
      </Group>
      <Group justify="space-between" align="center" gap="sm" wrap="wrap">
        <Text className="history-time" size="sm" c="dimmed"><IconClock size={15} /> {new Date(entry.play.watched_at).toLocaleTimeString(undefined, { hour: "numeric", minute: "2-digit" })}</Text>
        <Group gap="xs">
          <Button size="xs" variant="subtle" leftSection={<IconEdit size={14} />} onClick={() => setEditing(value => !value)}>{editing ? "Cancel" : "Edit time"}</Button>
          <Button size="xs" variant="light" leftSection={<IconRefresh size={14} />} loading={rewatch.isPending} onClick={() => rewatch.mutate()}>Rewatch</Button>
          <Button size="xs" color="red" variant="subtle" leftSection={<IconTrash size={14} />} loading={remove.isPending} onClick={() => { if (window.confirm("Delete this individual watch?")) remove.mutate(); }}>Delete</Button>
        </Group>
      </Group>
      {editing && <Group className="history-edit" align="end" gap="xs"><TextInput type="datetime-local" label="Watched at" aria-label={`Watched time for ${entry.title}`} value={watchedAt} onChange={event => setWatchedAt(event.currentTarget.value)} /><Button size="sm" disabled={!watchedAt} loading={save.isPending} onClick={() => save.mutate()}>Save time</Button></Group>}
      {(save.isError || rewatch.isError || remove.isError) && <Alert color="red">{save.error?.message || rewatch.error?.message || remove.error?.message}</Alert>}
    </Stack>
  </Paper>;
}
