import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Alert,
  Anchor,
  Badge,
  Button,
  Code,
  Group,
  Modal,
  ScrollArea,
  Stack,
  Table,
  Text,
  TextInput,
} from "@mantine/core";
import { IconCopy, IconRefresh, IconTrash } from "@tabler/icons-react";
import { useUserQueryKey } from "../auth/SessionContext";
import { ActivityTime } from "../../components/ActivityTime";
import { QueryError } from "../../components/QueryError";
import { SettingsSection, SettingsSubsection } from "./SettingsSection";

type PlexEvent = {
  id: number;
  event_type?: string;
  status: "synced" | "skipped" | "failed";
  title?: string;
  media_type?: string;
  message?: string;
  occurred_at: string;
};

type PlexStatus = {
  mode: "personal" | "managed";
  managed_account_id?: string;
  managed_webhook_enabled: boolean;
  enabled: boolean;
  account_id?: string;
  created_at?: string;
  last_used_at?: string;
  last_synced_at?: string;
  recent_events: PlexEvent[];
};

function PayloadModal({ eventId, onClose }: { eventId: number | null; onClose: () => void }) {
  const payload = useQuery({
    queryKey: ["plex-webhook-event", eventId],
    enabled: eventId !== null,
    queryFn: async () => {
      const response = await fetch(`/api/v1/profile/plex-webhook/events/${eventId}`);
      if (!response.ok) throw new Error("The Plex payload could not be loaded.");
      const result = (await response.json()) as { raw_payload: string };
      try {
        return JSON.stringify(JSON.parse(result.raw_payload), null, 2);
      } catch {
        return result.raw_payload;
      }
    },
  });
  const [copyError, setCopyError] = useState("");
  return (
    <Modal opened={eventId !== null} onClose={onClose} title="Plex payload" size="lg">
      {payload.isError && <Alert color="red">{payload.error.message}</Alert>}
      {payload.isSuccess && !payload.data && (
        <Text size="sm" c="dimmed">
          No payload was stored for this event.
        </Text>
      )}
      {payload.data && (
        <Stack gap="xs">
          {copyError && <Alert color="red">{copyError}</Alert>}
          <Group justify="flex-end">
            <Button
              variant="default"
              size="xs"
              leftSection={<IconCopy size={14} />}
              onClick={() =>
                navigator.clipboard
                  .writeText(payload.data)
                  .then(() => setCopyError(""))
                  .catch(() => setCopyError("Could not copy the payload. Select and copy it."))
              }
            >
              Copy
            </Button>
          </Group>
          <ScrollArea h={400}>
            <Code block>{payload.data}</Code>
          </ScrollArea>
        </Stack>
      )}
    </Modal>
  );
}

export function PlexSyncPanel() {
  const [payloadEventId, setPayloadEventId] = useState<number | null>(null);
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const [issuedURL, setIssuedURL] = useState("");
  const [copyError, setCopyError] = useState("");
  const [accountID, setAccountID] = useState("");
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
      const response = await fetch("/api/v1/profile/plex-webhook", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ account_id: accountID.trim() || status.data?.account_id || "" }),
      });
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

  return (
    <SettingsSection
      id="plex"
      title="Plex watch sync"
      description={
        status.data?.mode === "managed"
          ? "Your administrator manages one Plex webhook for this instance. Your watches sync when your Plex account is assigned below."
          : "Sync new watched movies and episodes from your Plex account. Plex Pass and a public HTTPS address for this Visto instance are required."
      }
    >
      {status.data?.mode === "personal" && (
        <Text size="sm" c="dimmed" mt="xs">
          After you create a webhook URL, add it in your{" "}
          <Anchor
            href="https://app.plex.tv/desktop/#!/settings/webhooks"
            target="_blank"
            rel="noopener noreferrer"
          >
            Plex webhook settings
          </Anchor>
          .
        </Text>
      )}
      {status.data?.mode === "managed" && (
        <Text size="sm" mt="md">
          {status.data.managed_account_id && status.data.managed_webhook_enabled
            ? `Plex account ID ${status.data.managed_account_id} is assigned to you and the shared webhook is active.`
            : status.data.managed_account_id
              ? `Plex account ID ${status.data.managed_account_id} is assigned to you. The shared webhook is not active.`
              : "No Plex account is assigned to you yet. Ask an administrator to assign one."}
        </Text>
      )}
      {status.isError && (
        <QueryError message={status.error.message} onRetry={() => status.refetch()} />
      )}
      {issue.isError && (
        <Alert color="red" mt="md">
          {issue.error.message}
        </Alert>
      )}
      {revoke.isError && (
        <Alert color="red" mt="md">
          {revoke.error.message}
        </Alert>
      )}
      {status.data?.mode === "personal" && status.data?.enabled && (
        <Text size="sm" mt="md">
          Webhook active
          {status.data.created_at && (
            <>
              {" "}
              since <ActivityTime value={status.data.created_at} />
            </>
          )}
          .
          {status.data.account_id
            ? ` Plex account ID ${status.data.account_id}.`
            : " No Plex account selected; all watches are skipped."}
          {status.data.last_synced_at ? (
            <>
              {" "}
              Last successful sync <ActivityTime value={status.data.last_synced_at} />.
            </>
          ) : (
            " No watched events synced yet."
          )}
        </Text>
      )}
      {status.data?.mode === "personal" && (
        <>
          <TextInput
            mt="md"
            label="Plex account ID"
            description="Enter your numeric Plex account ID. To find it, create a URL with this field empty, watch something in Plex, then check Recent sync activity for the skipped account ID. Enter that ID and rotate the URL."
            placeholder={status.data?.account_id || "Plex account ID"}
            value={accountID}
            onChange={(event) => setAccountID(event.currentTarget.value)}
            inputMode="numeric"
          />
          {issuedURL && (
            <Stack gap="xs" mt="md">
              <Alert color="yellow" title="Copy this URL into Plex now">
                Visto only shows the secret URL once. If you leave this page, rotate the URL to get
                a new one.
              </Alert>
              {copyError && <Alert color="red">{copyError}</Alert>}
              <Group align="end" wrap="nowrap">
                <Code
                  style={{ flex: 1, overflowWrap: "anywhere", whiteSpace: "normal", padding: 10 }}
                >
                  {issuedURL}
                </Code>
                <Button
                  variant="default"
                  leftSection={<IconCopy size={16} />}
                  onClick={() => {
                    navigator.clipboard
                      .writeText(issuedURL)
                      .then(() => setCopyError(""))
                      .catch(() =>
                        setCopyError("Could not copy the URL. Select and copy it instead."),
                      );
                  }}
                >
                  Copy URL
                </Button>
              </Group>
            </Stack>
          )}
          <Group mt="md">
            <Button
              loading={issue.isPending}
              leftSection={status.data?.enabled ? <IconRefresh size={16} /> : undefined}
              onClick={() => {
                if (
                  status.data?.enabled &&
                  !window.confirm("Rotate your Plex webhook URL? The old URL will stop working.")
                )
                  return;
                issue.mutate();
              }}
            >
              {status.data?.enabled ? "Rotate webhook URL" : "Create webhook URL"}
            </Button>
            {status.data?.enabled && (
              <Button
                color="red"
                variant="subtle"
                loading={revoke.isPending}
                leftSection={<IconTrash size={16} />}
                onClick={() => {
                  if (
                    window.confirm(
                      "Revoke your Plex webhook? Plex will stop syncing watched content.",
                    )
                  )
                    revoke.mutate();
                }}
              >
                Revoke
              </Button>
            )}
          </Group>
        </>
      )}
      {(status.data?.recent_events.length ?? 0) > 0 && (
        <SettingsSubsection title="Recent sync activity">
          <Table.ScrollContainer minWidth={620} mt="sm">
            <Table striped highlightOnHover withTableBorder>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>Media</Table.Th>
                  <Table.Th>Event</Table.Th>
                  <Table.Th>Status</Table.Th>
                  <Table.Th>Details</Table.Th>
                  <Table.Th>Time</Table.Th>
                  <Table.Th />
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {status.data!.recent_events.map((event) => (
                  <Table.Tr key={event.id}>
                    <Table.Td>{event.title || event.media_type || "Plex event"}</Table.Td>
                    <Table.Td>{event.event_type || "—"}</Table.Td>
                    <Table.Td>
                      <Badge
                        color={
                          event.status === "synced"
                            ? "green"
                            : event.status === "failed"
                              ? "red"
                              : "gray"
                        }
                      >
                        {event.status}
                      </Badge>
                    </Table.Td>
                    <Table.Td>{event.message || "—"}</Table.Td>
                    <Table.Td>
                      <ActivityTime value={event.occurred_at} />
                    </Table.Td>
                    <Table.Td>
                      <Button
                        variant="subtle"
                        size="compact-xs"
                        onClick={() => setPayloadEventId(event.id)}
                      >
                        View payload
                      </Button>
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>
        </SettingsSubsection>
      )}
      <PayloadModal eventId={payloadEventId} onClose={() => setPayloadEventId(null)} />
    </SettingsSection>
  );
}
