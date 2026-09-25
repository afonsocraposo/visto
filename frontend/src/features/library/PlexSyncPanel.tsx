import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Badge, Button, Code, Group, Paper, Stack, Table, Text, Title } from "@mantine/core";
import { IconCopy, IconRefresh, IconTrash } from "@tabler/icons-react";
import { useUserQueryKey } from "../auth/SessionContext";
import { formatActivityTime } from "../../lib/time";

type PlexEvent = {
  id: number;
  status: "synced" | "skipped" | "failed";
  title?: string;
  media_type?: string;
  message?: string;
  occurred_at: string;
};

type PlexStatus = {
  enabled: boolean;
  created_at?: string;
  last_used_at?: string;
  last_synced_at?: string;
  recent_events: PlexEvent[];
};

export function PlexSyncPanel() {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const [issuedURL, setIssuedURL] = useState("");
  const [copyError, setCopyError] = useState("");
  const key = userQueryKey("plex-webhook");
  const status = useQuery({
    queryKey: key,
    queryFn: async () => {
      const response = await fetch("/api/v1/profile/plex-webhook");
      if (!response.ok) throw new Error("Plex sync settings could not be loaded.");
      return response.json() as Promise<PlexStatus>;
    },
  });
  const issue = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/profile/plex-webhook", { method: "POST" });
      const result = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(result.error || "Could not create a Plex webhook URL.");
      return result.webhook_url as string;
    },
    onSuccess: async (url) => {
      setIssuedURL(url);
      setCopyError("");
      await queryClient.invalidateQueries({ queryKey: key });
    },
  });
  const revoke = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/profile/plex-webhook", { method: "DELETE" });
      if (!response.ok) throw new Error("Could not revoke the Plex webhook URL.");
    },
    onSuccess: async () => {
      setIssuedURL("");
      await queryClient.invalidateQueries({ queryKey: key });
    },
  });

  return <Paper withBorder p="md" mt="lg">
    <Title order={3}>Plex watch sync</Title>
    <Text size="sm" c="dimmed" mt="xs">
      Sync new watched movies and episodes from your Plex account. Plex Pass and a public HTTPS address for this Visto instance are required.
    </Text>
    {status.isError && <Alert color="red" mt="md">{status.error.message}</Alert>}
    {issue.isError && <Alert color="red" mt="md">{issue.error.message}</Alert>}
    {revoke.isError && <Alert color="red" mt="md">{revoke.error.message}</Alert>}
    {status.data?.enabled && <Text size="sm" mt="md">
      Webhook active{status.data.created_at ? ` since ${formatActivityTime(status.data.created_at)}` : ""}.
      {status.data.last_synced_at ? ` Last successful sync ${formatActivityTime(status.data.last_synced_at)}.` : " No watched events synced yet."}
    </Text>}
    {issuedURL && <Stack gap="xs" mt="md">
      <Alert color="yellow" title="Copy this URL into Plex now">
        Visto only shows the secret URL once. If you leave this page, rotate the URL to get a new one.
      </Alert>
      {copyError && <Alert color="red">{copyError}</Alert>}
      <Group align="end" wrap="nowrap">
        <Code style={{ flex: 1, overflowWrap: "anywhere", whiteSpace: "normal", padding: 10 }}>{issuedURL}</Code>
        <Button variant="default" leftSection={<IconCopy size={16} />} onClick={() => {
          navigator.clipboard.writeText(issuedURL).then(() => setCopyError("")).catch(() => setCopyError("Could not copy the URL. Select and copy it instead."));
        }}>Copy URL</Button>
      </Group>
    </Stack>}
    <Group mt="md">
      <Button loading={issue.isPending} leftSection={status.data?.enabled ? <IconRefresh size={16} /> : undefined}
        onClick={() => {
          if (status.data?.enabled && !window.confirm("Rotate your Plex webhook URL? The old URL will stop working.")) return;
          issue.mutate();
        }}>
        {status.data?.enabled ? "Rotate webhook URL" : "Create webhook URL"}
      </Button>
      {status.data?.enabled && <Button color="red" variant="subtle" loading={revoke.isPending} leftSection={<IconTrash size={16} />}
        onClick={() => {
          if (window.confirm("Revoke your Plex webhook? Plex will stop syncing watched content.")) revoke.mutate();
        }}>Revoke</Button>}
    </Group>
    {(status.data?.recent_events.length ?? 0) > 0 && <>
      <Title order={4} mt="xl">Recent sync activity</Title>
      <Table mt="sm" striped highlightOnHover withTableBorder>
        <Table.Thead><Table.Tr><Table.Th>Media</Table.Th><Table.Th>Status</Table.Th><Table.Th>Details</Table.Th><Table.Th>Time</Table.Th></Table.Tr></Table.Thead>
        <Table.Tbody>{status.data!.recent_events.map((event) => <Table.Tr key={event.id}>
          <Table.Td>{event.title || event.media_type || "Plex event"}</Table.Td>
          <Table.Td><Badge color={event.status === "synced" ? "green" : event.status === "failed" ? "red" : "gray"}>{event.status}</Badge></Table.Td>
          <Table.Td>{event.message || "—"}</Table.Td>
          <Table.Td>{formatActivityTime(event.occurred_at)}</Table.Td>
        </Table.Tr>)}</Table.Tbody>
      </Table>
    </>}
  </Paper>;
}
