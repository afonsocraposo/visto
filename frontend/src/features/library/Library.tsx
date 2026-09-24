import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Group, Loader, Paper, PasswordInput, Select, Text, TextInput, Title } from "@mantine/core";
import { EmptyState } from "../../components/EmptyState";
import type { HistoryEntry, LibraryEntry, User } from "../../types";

export function LibraryPanel() {
  const library = useQuery({
    queryKey: ["library"],
    queryFn: async () => {
      const response = await fetch("/api/v1/library");
      if (!response.ok) throw new Error();
      return response.json() as Promise<LibraryEntry[]>;
    },
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

function LibraryCard({ entry }: { entry: LibraryEntry }) {
  const queryClient = useQueryClient();
  const update = useMutation({
    mutationFn: async ({ status, rating }: { status: string; rating: number | null }) => {
      const response = await fetch(`/api/v1/library/${encodeURIComponent(entry.item.media_id)}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ status, rating }),
      });
      if (!response.ok) throw new Error("Could not update this title.");
    },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["library"] }),
        queryClient.invalidateQueries({ queryKey: ["continue"] }),
        queryClient.invalidateQueries({ queryKey: ["calendar"] }),
        queryClient.invalidateQueries({ queryKey: ["feed"] }),
      ]);
    },
  });
  const watched = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/plays", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ media_id: entry.item.media_id }),
      });
      if (!response.ok) throw new Error("Could not record this watch.");
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["history"] }),
  });
  const setStatus = (status: string | null) => {
    if (status) update.mutate({ status, rating: entry.item.rating });
  };
  const setRating = (value: string | null) => {
    update.mutate({ status: entry.item.status, rating: value && value !== "none" ? Number(value) : null });
  };

  return (
    <Paper withBorder p="md" mt="sm">
      <Group justify="space-between">
        <Text fw={700}>{entry.media.title}</Text>
        <Text size="sm" c="dimmed">{entry.media.type === "tv" ? "TV show" : "Movie"}</Text>
      </Group>
      <Group mt="sm" grow>
        <Select aria-label={`Status for ${entry.media.title}`} value={entry.item.status} onChange={setStatus} data={[
          { value: "watchlist", label: "Watchlist" },
          { value: "watching", label: "Watching" },
          { value: "paused", label: "Paused" },
          { value: "dropped", label: "Dropped" },
        ]} />
        <Select aria-label={`Rating for ${entry.media.title}`} value={entry.item.rating ? String(entry.item.rating) : "none"} onChange={setRating} data={[
          { value: "none", label: "Not rated" },
          ...[1, 2, 3, 4, 5].map(value => ({ value: String(value), label: `${value} / 5 stars` })),
        ]} />
      </Group>
      {entry.media.type === "movie" && <Button mt="sm" size="xs" loading={watched.isPending} onClick={() => watched.mutate()}>Watched</Button>}
      {update.isError && <Alert color="red" mt="sm">{update.error.message}</Alert>}
      {watched.isError && <Alert color="red" mt="sm">{watched.error.message}</Alert>}
    </Paper>
  );
}

export function HistoryPanel() {
  const history = useQuery({
    queryKey: ["history"],
    queryFn: async () => {
      const response = await fetch("/api/v1/plays?limit=100");
      if (!response.ok) throw new Error();
      return response.json() as Promise<HistoryEntry[]>;
    },
  });
  if (history.isPending) return <Group justify="center" mt="xl"><Loader /></Group>;
  if (history.isError) return <Alert color="red" mt="lg">Watch history is temporarily unavailable.</Alert>;

  return (
    <Paper withBorder p="md" mt="lg">
      <Title order={2}>Watch history</Title>
      {history.data.length === 0
        ? <Text c="dimmed" mt="sm">No watches recorded yet.</Text>
        : history.data.map(entry => <HistoryCard key={entry.play.id} entry={entry} />)}
    </Paper>
  );
}

function HistoryCard({ entry }: { entry: HistoryEntry }) {
  const queryClient = useQueryClient();
  const [watchedAt, setWatchedAt] = useState(() => {
    const date = new Date(entry.play.watched_at);
    return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
  });
  const refreshHistory = () => Promise.all([
    queryClient.invalidateQueries({ queryKey: ["history"] }),
    queryClient.invalidateQueries({ queryKey: ["continue"] }),
    queryClient.invalidateQueries({ queryKey: ["calendar"] }),
    queryClient.invalidateQueries({ queryKey: ["feed"] }),
  ]);
  const save = useMutation({
    mutationFn: async () => {
      const response = await fetch(`/api/v1/plays/${encodeURIComponent(entry.play.id)}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ watched_at: new Date(watchedAt).toISOString() }),
      });
      if (!response.ok) throw new Error("Could not correct this watch.");
    },
    onSuccess: refreshHistory,
  });
  const remove = useMutation({
    mutationFn: async () => {
      const response = await fetch(`/api/v1/plays/${encodeURIComponent(entry.play.id)}`, { method: "DELETE" });
      if (!response.ok) throw new Error("Could not delete this watch.");
    },
    onSuccess: refreshHistory,
  });
  const rewatch = useMutation({
    mutationFn: async () => {
      const body = entry.play.media_id ? { media_id: entry.play.media_id } : { episode_id: entry.play.episode_id };
      const response = await fetch("/api/v1/plays", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!response.ok) throw new Error("Could not record the rewatch.");
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["history"] }),
  });

  return (
    <Group justify="space-between" align="end" mt="md" wrap="wrap">
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
    </Group>
  );
}

export function ProfilePanel({ user }: { user: User }) {
  const queryClient = useQueryClient();
  const [visibility, setVisibility] = useState("private");
  const [timezone, setTimezone] = useState("UTC");
  const settings = useQuery({
    queryKey: ["profile-settings"],
    queryFn: async () => {
      const response = await fetch("/api/v1/profile/activity-settings");
      if (!response.ok) throw new Error();
      return response.json() as Promise<{ activity_visibility: string; timezone: string }>;
    },
  });
  useEffect(() => {
    if (settings.data) {
      setVisibility(settings.data.activity_visibility);
      setTimezone(settings.data.timezone);
    }
  }, [settings.data]);
  const save = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/profile/activity-settings", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ activity_visibility: visibility, timezone }),
      });
      if (!response.ok) throw new Error("Could not save settings.");
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["profile-settings"] }),
  });

  return (
    <>
      <Paper withBorder p="md" mt="lg">
        <Title order={2}>Profile</Title>
        <Text size="sm" c="dimmed" mt="xs">Choose who can see your activity and which time zone the calendar uses.</Text>
        {settings.isError && <Alert color="red" mt="md">Profile settings are temporarily unavailable.</Alert>}
        <Select mt="md" label="Activity feed" value={visibility} onChange={value => setVisibility(value || "private")} data={[
          { value: "private", label: "Private" },
          { value: "instance", label: "Visible to this instance" },
        ]} />
        <TextInput mt="md" label="Time zone" description="Use a time zone such as Europe/Lisbon or America/New_York." value={timezone} onChange={event => setTimezone(event.currentTarget.value)} />
        {save.isError && <Alert color="red" mt="md">{save.error.message}</Alert>}
        <Button mt="md" loading={save.isPending} onClick={() => save.mutate()}>Save settings</Button>
      </Paper>
      {user.role === "admin" && <CreateUserPanel />}
    </>
  );
}

function CreateUserPanel() {
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [created, setCreated] = useState("");
  const create = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/users", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, display_name: displayName, password }),
      });
      if (!response.ok) {
        const result = await response.json().catch(() => ({}));
        throw new Error(result.error || "Could not create this account.");
      }
      return response.json() as Promise<User>;
    },
    onSuccess: user => {
      setCreated(user.display_name);
      setUsername("");
      setDisplayName("");
      setPassword("");
    },
  });
  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setCreated("");
    await create.mutateAsync();
  };

  return (
    <Paper withBorder p="md" mt="lg">
      <Title order={2}>Add a family member</Title>
      <Text size="sm" c="dimmed" mt="xs">Each person gets a separate library and watch history.</Text>
      <form onSubmit={event => void submit(event).catch(() => {})}>
        <TextInput required minLength={3} maxLength={32} label="Username" value={username} onChange={event => setUsername(event.currentTarget.value)} mt="md" />
        <TextInput required maxLength={80} label="Name" value={displayName} onChange={event => setDisplayName(event.currentTarget.value)} mt="md" />
        <PasswordInput required minLength={12} label="Temporary password" value={password} onChange={event => setPassword(event.currentTarget.value)} mt="md" />
        {create.isError && <Alert color="red" mt="md">{create.error.message}</Alert>}
        {created && <Alert color="green" mt="md">Account created for {created}.</Alert>}
        <Button type="submit" loading={create.isPending} mt="md">Create account</Button>
      </form>
    </Paper>
  );
}
