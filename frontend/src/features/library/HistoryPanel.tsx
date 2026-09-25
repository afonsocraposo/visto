import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "@mantine/form";
import { ActionIcon, Alert, Button, Group, Loader, Menu, Text, TextInput } from "@mantine/core";
import { IconClock, IconDots, IconEdit, IconRefresh, IconTrash } from "@tabler/icons-react";
import { EmptyState } from "../../components/EmptyState";
import { ActivityRow } from "../feed/ActivityRow";
import { useUserQueryKey } from "../auth/SessionContext";
import { api } from "../../lib/api";
import type { HistoryEntry } from "../../types";

export function HistoryPanel() {
  const userQueryKey = useUserQueryKey();
  const history = useQuery({
    queryKey: userQueryKey("history"),
    queryFn: () =>
      api.get<HistoryEntry[]>(
        "/api/v1/plays?limit=100",
        "Watch history is temporarily unavailable.",
      ),
  });
  if (history.isPending)
    return (
      <Group justify="center" mt="xl">
        <Loader />
      </Group>
    );
  if (history.isError)
    return (
      <Alert color="red" mt="lg">
        Watch history is temporarily unavailable.
      </Alert>
    );

  return (
    <div className="activity-list">
      {history.data.length === 0 ? (
        <EmptyState
          title="No watches recorded yet"
          detail="Movies and episodes you watch will appear here."
        />
      ) : (
        history.data.map((entry) => <HistoryCard key={entry.play.id} entry={entry} />)
      )}
    </div>
  );
}

function HistoryCard({ entry }: { entry: HistoryEntry }) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const [editing, setEditing] = useState(false);
  const watchedAt = () => {
    const date = new Date(entry.play.watched_at);
    return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
  };
  const form = useForm({ initialValues: { watchedAt: watchedAt() } });
  const refreshHistory = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("history") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("continue") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("show-progress") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("calendar") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
    ]);
  const save = useMutation({
    mutationFn: async () => {
      await api.patch(
        `/api/v1/plays/${encodeURIComponent(entry.play.id)}`,
        { watched_at: new Date(form.values.watchedAt).toISOString() },
        "Could not correct this watch.",
      );
    },
    onSuccess: refreshHistory,
  });
  const remove = useMutation({
    mutationFn: async () => {
      await api.delete(
        `/api/v1/plays/${encodeURIComponent(entry.play.id)}`,
        "Could not delete this watch.",
      );
    },
    onSuccess: refreshHistory,
  });
  const rewatch = useMutation({
    mutationFn: async () => {
      const body = entry.play.media_id
        ? { media_id: entry.play.media_id }
        : { episode_id: entry.play.episode_id };
      await api.post("/api/v1/plays", body, "Could not record the rewatch.");
    },
    onSuccess: async () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: userQueryKey("history") }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("library") }),
        queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
      ]),
  });

  const watchedAtDate = new Date(entry.play.watched_at);
  const mediaType = entry.play.episode_id ? "tv" : "movie";
  const actions = (
    <Menu withinPortal position="bottom-end">
      <Menu.Target>
        <ActionIcon
          variant="subtle"
          aria-label={`Actions for ${entry.episode_name || entry.title}`}
        >
          <IconDots size={19} />
        </ActionIcon>
      </Menu.Target>
      <Menu.Dropdown>
        <Menu.Item
          leftSection={<IconEdit size={15} />}
          onClick={() => {
            if (!editing) form.setFieldValue("watchedAt", watchedAt());
            setEditing((value) => !value);
          }}
        >
          {editing ? "Cancel edit" : "Edit watch time"}
        </Menu.Item>
        <Menu.Item
          leftSection={<IconRefresh size={15} />}
          disabled={rewatch.isPending}
          onClick={() => rewatch.mutate()}
        >
          Rewatch
        </Menu.Item>
        <Menu.Divider />
        <Menu.Item
          color="red"
          leftSection={<IconTrash size={15} />}
          disabled={remove.isPending}
          onClick={() => {
            if (window.confirm("Delete this individual watch?")) remove.mutate();
          }}
        >
          Delete watch
        </Menu.Item>
      </Menu.Dropdown>
    </Menu>
  );

  return (
    <ActivityRow
      title={entry.title}
      mediaType={mediaType}
      artworkPath={entry.artwork_path}
      action="watched"
      episodeLabel={entry.episode_label}
      episodeName={entry.episode_name}
      occurredAt={watchedAtDate}
      exactTime={
        <>
          <IconClock size={13} />{" "}
          {watchedAtDate.toLocaleTimeString(undefined, { hour: "numeric", minute: "2-digit" })}
        </>
      }
      actions={actions}
    >
      {editing && (
        <form className="activity-edit" onSubmit={form.onSubmit(() => save.mutate())}>
          <Group align="end" gap="xs" wrap="wrap">
            <TextInput
              type="datetime-local"
              label="Watched at"
              aria-label={`Watched time for ${entry.title}`}
              {...form.getInputProps("watchedAt")}
            />
            <Button
              type="submit"
              size="sm"
              disabled={!form.values.watchedAt}
              loading={save.isPending}
            >
              Save time
            </Button>
            <Button type="button" size="sm" variant="subtle" onClick={() => setEditing(false)}>
              Cancel
            </Button>
          </Group>
        </form>
      )}
      {(save.isError || rewatch.isError || remove.isError) && (
        <Alert color="red" mt="sm">
          {save.error?.message || rewatch.error?.message || remove.error?.message}
        </Alert>
      )}
      {(rewatch.isPending || remove.isPending) && (
        <Text size="xs" c="dimmed" mt="xs">
          {rewatch.isPending ? "Recording rewatch…" : "Deleting watch…"}
        </Text>
      )}
    </ActivityRow>
  );
}
