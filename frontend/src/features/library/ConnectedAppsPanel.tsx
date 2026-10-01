import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Group, Modal, Text } from "@mantine/core";
import { IconTrash } from "@tabler/icons-react";
import { api } from "../../lib/api";
import { useUserQueryKey } from "../auth/SessionContext";
import type { ConnectedApp } from "../../types";
import { QueryError } from "../../components/QueryError";
import { SettingsListSkeleton, SettingsSection } from "./SettingsSection";

export function ConnectedAppsPanel() {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const [pendingRevoke, setPendingRevoke] = useState<ConnectedApp | "all" | null>(null);
  const connections = useQuery({
    queryKey: userQueryKey("connected-apps"),
    queryFn: () =>
      api.get<ConnectedApp[]>("/api/v1/connected-apps", "Could not load connected apps."),
  });
  const revoke = useMutation({
    mutationFn: (clientID: string) =>
      api.delete(
        `/api/v1/connected-apps/${encodeURIComponent(clientID)}`,
        "Could not revoke connected app.",
      ),
    onSuccess: async () => {
      setPendingRevoke(null);
      await queryClient.invalidateQueries({ queryKey: userQueryKey("connected-apps") });
    },
  });
  const revokeAll = useMutation({
    mutationFn: () => api.delete("/api/v1/connected-apps", "Could not revoke connected apps."),
    onSuccess: async () => {
      setPendingRevoke(null);
      await queryClient.invalidateQueries({ queryKey: userQueryKey("connected-apps") });
    },
  });

  return (
    <SettingsSection
      id="apps"
      title="Connected apps"
      description="Apps connected with OAuth can access only the permissions you approved. Revoke access at any time."
    >
      {connections.isError && (
        <QueryError message={connections.error.message} onRetry={() => connections.refetch()} />
      )}
      {revoke.isError && (
        <Alert color="red" mt="md">
          {revoke.error.message}
        </Alert>
      )}
      <Modal
        opened={pendingRevoke !== null}
        onClose={() => setPendingRevoke(null)}
        title="Revoke connected app?"
        centered
      >
        <Text size="sm">
          {pendingRevoke === "all"
            ? "Every connected app will immediately lose access to your Visto account."
            : `${pendingRevoke?.client_name} will immediately lose access to your Visto account.`}
        </Text>
        <Group justify="flex-end" mt="lg">
          <Button variant="default" onClick={() => setPendingRevoke(null)}>
            Cancel
          </Button>
          <Button
            color="red"
            loading={revoke.isPending || revokeAll.isPending}
            onClick={() =>
              pendingRevoke === "all"
                ? revokeAll.mutate()
                : pendingRevoke && revoke.mutate(pendingRevoke.client_id)
            }
          >
            Revoke access
          </Button>
        </Group>
      </Modal>
      {revokeAll.isError && (
        <Alert color="red" mt="md">
          {revokeAll.error.message}
        </Alert>
      )}
      {connections.isPending ? (
        <SettingsListSkeleton />
      ) : connections.data?.length ? (
        <>
          <ul className="settings-list">
            {connections.data.map((connection) => (
              <li key={connection.client_id} className="settings-list-row">
                <div className="settings-list-text">
                  <Text fw={650} className="settings-list-title">
                    {connection.client_name}
                  </Text>
                  <Text size="xs" c="dimmed" className="settings-list-meta">
                    Can {scopeLabel(connection.scopes)} ·{" "}
                    {connection.last_used_at
                      ? `Last used ${formatDate(connection.last_used_at)}`
                      : connection.connected_at
                        ? `Connected ${formatDate(connection.connected_at)}`
                        : "Connected before activity tracking"}
                  </Text>
                </div>
                <Button
                  variant="subtle"
                  color="red"
                  aria-label={`Revoke ${connection.client_name}`}
                  leftSection={<IconTrash size={16} />}
                  onClick={() => setPendingRevoke(connection)}
                >
                  Revoke
                </Button>
              </li>
            ))}
          </ul>
          {connections.data.length > 1 && (
            <Group justify="flex-end" mt="sm">
              <Button
                variant="subtle"
                color="red"
                size="xs"
                onClick={() => setPendingRevoke("all")}
              >
                Revoke all
              </Button>
            </Group>
          )}
        </>
      ) : (
        <Text size="sm" c="dimmed" mt="md">
          You have no connected apps.
        </Text>
      )}
    </SettingsSection>
  );
}

function scopeLabel(scopes: string[]): string {
  const permissions = [];
  if (scopes.includes("read")) permissions.push("read your data");
  if (scopes.includes("write")) permissions.push("make changes");
  return permissions.join(" and ") || "access your data";
}

function formatDate(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? value
    : date.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}
