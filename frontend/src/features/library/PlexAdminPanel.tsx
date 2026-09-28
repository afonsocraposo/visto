import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Alert,
  Anchor,
  Button,
  Code,
  Group,
  Paper,
  Select,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import type { User } from "../../types";
import { ActivityTime } from "../../components/ActivityTime";
import { useUserQueryKey } from "../auth/SessionContext";
import { fetchAllPages } from "../../lib/pagination";

type PlexAdminStatus = {
  mode: "personal" | "managed";
  webhook_enabled: boolean;
  created_at?: string;
  last_used_at?: string;
  mappings: { user_id: string; account_id: string }[];
  observed_accounts: {
    account_id: string;
    title: string;
    last_seen_at: string;
    user_id?: string;
  }[];
};

async function request(path: string, method: string, body?: object) {
  const response = await fetch(`/api/v1/admin/plex-sync${path}`, {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || "Could not update Plex sync settings.");
  }
  return response;
}

export function PlexAdminPanel() {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const key = userQueryKey("admin-plex-sync");
  const usersQuery = useQuery({
    queryKey: userQueryKey("plex-assignment-users"),
    queryFn: () => fetchAllPages<User>("/api/v1/users", "Could not load users."),
  });
  const users = usersQuery.data ?? [];
  const [issuedURL, setIssuedURL] = useState("");
  const [accountID, setAccountID] = useState("");
  const [userID, setUserID] = useState<string | null>(null);
  const status = useQuery({
    queryKey: key,
    queryFn: async () => (await request("", "GET")).json() as Promise<PlexAdminStatus>,
  });
  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: key }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("plex-webhook") }),
    ]);
  };
  const issue = useMutation({
    mutationFn: async () =>
      (await request("/webhook", "POST")).json() as Promise<{ webhook_url: string }>,
    onSuccess: async (data) => {
      setIssuedURL(data.webhook_url);
      await refresh();
    },
  });
  const revoke = useMutation({
    mutationFn: () => request("/webhook", "DELETE"),
    onSuccess: async () => {
      setIssuedURL("");
      await refresh();
    },
  });
  const saveMapping = useMutation({
    mutationFn: ({ targetUserID, plexID }: { targetUserID: string; plexID: string }) =>
      request(`/users/${encodeURIComponent(targetUserID)}`, "PUT", { account_id: plexID }),
    onSuccess: async () => {
      setAccountID("");
      setUserID(null);
      await refresh();
    },
  });
  const removeMapping = useMutation({
    mutationFn: (targetUserID: string) =>
      request(`/users/${encodeURIComponent(targetUserID)}`, "DELETE"),
    onSuccess: refresh,
  });
  const error =
    status.error ||
    usersQuery.error ||
    issue.error ||
    revoke.error ||
    saveMapping.error ||
    removeMapping.error;
  const userName = (id: string) => users.find((user) => user.id === id)?.name || `User ${id}`;

  return (
    <Paper withBorder p="md">
      <Title order={2}>Plex watch sync</Title>
      <Text size="sm" c="dimmed" mt="xs">
        Mode: {status.data?.mode === "managed" ? "Admin-managed" : "Personal"}. Set
        VISTO_PLEX_SYNC_MODE in Docker Compose to change it.
      </Text>
      {error && (
        <Alert color="red" mt="md">
          {error.message}
        </Alert>
      )}
      {status.data && (
        <Stack mt="md">
          {status.data.mode === "managed" && (
            <>
              <Text size="sm">
                Create one webhook URL and add it to your{" "}
                <Anchor
                  href="https://app.plex.tv/desktop/#!/settings/webhooks"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Plex webhook settings
                </Anchor>
                . Start playback on each account to discover its Plex ID below. Only mapped accounts
                sync watched content.
              </Text>
              <Group>
                <Button
                  loading={issue.isPending}
                  onClick={() => {
                    if (
                      !status.data?.webhook_enabled ||
                      window.confirm(
                        "Rotate the shared Plex webhook URL? The old URL will stop working.",
                      )
                    )
                      issue.mutate();
                  }}
                >
                  {status.data.webhook_enabled ? "Rotate shared URL" : "Create shared URL"}
                </Button>
                {status.data.webhook_enabled && (
                  <Button
                    color="red"
                    variant="subtle"
                    loading={revoke.isPending}
                    onClick={() => {
                      if (window.confirm("Revoke the shared Plex webhook URL? Sync will stop."))
                        revoke.mutate();
                    }}
                  >
                    Revoke
                  </Button>
                )}
              </Group>
              {issuedURL && (
                <Alert color="yellow" title="Copy this URL into Plex now">
                  <Code style={{ overflowWrap: "anywhere", whiteSpace: "normal" }}>
                    {issuedURL}
                  </Code>
                  <Button
                    ml="sm"
                    size="xs"
                    onClick={() => void navigator.clipboard.writeText(issuedURL)}
                  >
                    Copy URL
                  </Button>
                </Alert>
              )}
              {status.data.webhook_enabled && (
                <Text size="sm" c="dimmed">
                  Shared URL active
                  {status.data.last_used_at ? (
                    <>
                      {" "}
                      · Last callback <ActivityTime value={status.data.last_used_at} />
                    </>
                  ) : (
                    " · No callbacks yet"
                  )}
                </Text>
              )}
              <Title order={3}>Assign Plex accounts</Title>
              <Text size="sm" c="dimmed">
                Select an observed account or enter its numeric ID, then choose a Visto user.
              </Text>
              <Group align="end">
                <TextInput
                  label="Plex account ID"
                  value={accountID}
                  onChange={(event) => setAccountID(event.currentTarget.value)}
                  inputMode="numeric"
                />
                <Select
                  label="Visto user"
                  placeholder="Choose user"
                  data={users.map((user) => ({ value: user.id, label: user.name }))}
                  value={userID}
                  onChange={setUserID}
                  searchable
                />
                <Button
                  disabled={!userID || !/^\d{1,20}$/.test(accountID)}
                  loading={saveMapping.isPending}
                  onClick={() => {
                    if (userID) saveMapping.mutate({ targetUserID: userID, plexID: accountID });
                  }}
                >
                  Assign
                </Button>
              </Group>
              {status.data.mappings.length > 0 && (
                <Table striped withTableBorder>
                  <Table.Thead>
                    <Table.Tr>
                      <Table.Th>Visto user</Table.Th>
                      <Table.Th>Plex ID</Table.Th>
                      <Table.Th></Table.Th>
                    </Table.Tr>
                  </Table.Thead>
                  <Table.Tbody>
                    {status.data.mappings.map((mapping) => (
                      <Table.Tr key={mapping.user_id}>
                        <Table.Td>{userName(mapping.user_id)}</Table.Td>
                        <Table.Td>{mapping.account_id}</Table.Td>
                        <Table.Td>
                          <Button
                            size="xs"
                            color="red"
                            variant="subtle"
                            onClick={() => removeMapping.mutate(mapping.user_id)}
                          >
                            Remove
                          </Button>
                        </Table.Td>
                      </Table.Tr>
                    ))}
                  </Table.Tbody>
                </Table>
              )}
              <Title order={3}>Recently seen Plex accounts</Title>
              {status.data.observed_accounts.length === 0 ? (
                <Text size="sm" c="dimmed">
                  No Plex accounts seen yet. Start playback after adding the shared URL in Plex.
                </Text>
              ) : (
                <Table striped withTableBorder>
                  <Table.Thead>
                    <Table.Tr>
                      <Table.Th>Plex account</Table.Th>
                      <Table.Th>ID</Table.Th>
                      <Table.Th>Last seen</Table.Th>
                      <Table.Th>Assigned to</Table.Th>
                      <Table.Th></Table.Th>
                    </Table.Tr>
                  </Table.Thead>
                  <Table.Tbody>
                    {status.data.observed_accounts.map((account) => (
                      <Table.Tr key={account.account_id}>
                        <Table.Td>{account.title || "Unknown"}</Table.Td>
                        <Table.Td>{account.account_id}</Table.Td>
                        <Table.Td>
                          <ActivityTime value={account.last_seen_at} />
                        </Table.Td>
                        <Table.Td>
                          {account.user_id ? userName(account.user_id) : "Unassigned"}
                        </Table.Td>
                        <Table.Td>
                          <Button
                            size="xs"
                            variant="subtle"
                            onClick={() => setAccountID(account.account_id)}
                          >
                            Use ID
                          </Button>
                        </Table.Td>
                      </Table.Tr>
                    ))}
                  </Table.Tbody>
                </Table>
              )}
            </>
          )}
        </Stack>
      )}
    </Paper>
  );
}
