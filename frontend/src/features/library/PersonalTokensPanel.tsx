import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "@mantine/form";
import {
  Alert,
  Button,
  Code,
  CopyButton,
  Group,
  Modal,
  Text,
  TextInput,
  Tooltip,
} from "@mantine/core";
import { IconCheck, IconCopy, IconKey, IconTrash } from "@tabler/icons-react";
import { api } from "../../lib/api";
import { useUserQueryKey } from "../auth/SessionContext";
import type { IssuedPersonalAPIToken, PersonalAPIToken } from "../../types";
import { QueryError } from "../../components/QueryError";
import { SettingsListSkeleton, SettingsSection, SettingsSubsection } from "./SettingsSection";

export function PersonalTokensPanel() {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const form = useForm({ initialValues: { name: "", expiresAt: "" } });
  const [issued, setIssued] = useState<IssuedPersonalAPIToken | null>(null);
  const [pendingRevoke, setPendingRevoke] = useState<PersonalAPIToken | null>(null);
  const tokens = useQuery({
    queryKey: userQueryKey("personal-api-tokens"),
    queryFn: () => api.get<PersonalAPIToken[]>("/api/v1/tokens", "Could not load API tokens."),
  });
  const create = useMutation({
    mutationFn: () =>
      api.post<IssuedPersonalAPIToken>(
        "/api/v1/tokens",
        {
          name: form.values.name,
          ...(form.values.expiresAt
            ? { expires_at: new Date(form.values.expiresAt).toISOString() }
            : {}),
        },
        "Could not create API token.",
      ),
    onSuccess: async (token) => {
      setIssued(token);
      form.reset();
      await queryClient.invalidateQueries({ queryKey: userQueryKey("personal-api-tokens") });
    },
  });
  const revoke = useMutation({
    mutationFn: (tokenID: string) =>
      api.delete(`/api/v1/tokens/${encodeURIComponent(tokenID)}`, "Could not revoke API token."),
    onSuccess: async () => {
      setIssued(null);
      setPendingRevoke(null);
      await queryClient.invalidateQueries({ queryKey: userQueryKey("personal-api-tokens") });
    },
  });

  return (
    <SettingsSection
      id="tokens"
      title="Personal API tokens"
      description="Use a token to access your Visto data from scripts and integrations. Keep it private and revoke it if it is exposed."
    >
      {tokens.isError && (
        <QueryError message={tokens.error.message} onRetry={() => tokens.refetch()} />
      )}
      {issued && (
        <Alert color="green" mt="md" title="Copy your new token now">
          <Group justify="space-between" align="center" mb="xs">
            <Text size="sm">Visto will not show this token again.</Text>
            <CopyButton value={issued.token} timeout={1500}>
              {({ copied, copy }) => (
                <Tooltip label={copied ? "Copied" : "Copy token"} withArrow>
                  <Button
                    size="xs"
                    variant="default"
                    leftSection={copied ? <IconCheck size={15} /> : <IconCopy size={15} />}
                    onClick={() => void copy()}
                  >
                    {copied ? "Copied" : "Copy"}
                  </Button>
                </Tooltip>
              )}
            </CopyButton>
          </Group>
          <Code block>{issued.token}</Code>
        </Alert>
      )}
      {create.isError && (
        <Alert color="red" mt="md">
          {create.error.message}
        </Alert>
      )}
      {revoke.isError && (
        <Alert color="red" mt="md">
          {revoke.error.message}
        </Alert>
      )}
      <Modal
        opened={pendingRevoke !== null}
        onClose={() => setPendingRevoke(null)}
        title="Revoke API token?"
        centered
      >
        <Text size="sm">
          Apps using {pendingRevoke?.name} will lose access to your Visto account.
        </Text>
        <Group justify="flex-end" mt="lg">
          <Button variant="default" onClick={() => setPendingRevoke(null)}>
            Cancel
          </Button>
          <Button
            color="red"
            loading={revoke.isPending}
            onClick={() => pendingRevoke && revoke.mutate(pendingRevoke.id)}
          >
            Revoke token
          </Button>
        </Group>
      </Modal>
      <form
        onSubmit={form.onSubmit(() => {
          setIssued(null);
          create.mutate();
        })}
      >
        <TextInput
          required
          maxLength={80}
          label="Token name"
          placeholder="For example, home dashboard"
          mt="md"
          {...form.getInputProps("name")}
        />
        <TextInput
          type="datetime-local"
          label="Expires (optional)"
          description="Leave blank for a token that stays active until you revoke it."
          mt="md"
          {...form.getInputProps("expiresAt")}
        />
        <Button
          type="submit"
          leftSection={<IconKey size={16} />}
          loading={create.isPending}
          mt="md"
        >
          Create token
        </Button>
      </form>
      <SettingsSubsection title="Active tokens">
        {tokens.isPending ? (
          <SettingsListSkeleton />
        ) : tokens.data?.length ? (
          <ul className="settings-list">
            {tokens.data.map((token) => (
              <li key={token.id} className="settings-list-row">
                <div className="settings-list-text">
                  <Text fw={650} className="settings-list-title">
                    {token.name}
                  </Text>
                  <Text size="xs" c="dimmed" className="settings-list-meta">
                    Created {formatDate(token.created_at)} ·{" "}
                    {token.last_used_at
                      ? `Last used ${formatDate(token.last_used_at)}`
                      : "Not used yet"}
                    {token.expires_at
                      ? ` · Expires ${formatDate(token.expires_at)}`
                      : " · No expiry"}
                  </Text>
                </div>
                <Button
                  variant="subtle"
                  color="red"
                  aria-label={`Revoke ${token.name}`}
                  leftSection={<IconTrash size={16} />}
                  onClick={() => setPendingRevoke(token)}
                >
                  Revoke
                </Button>
              </li>
            ))}
          </ul>
        ) : (
          <Text size="sm" c="dimmed" mt="xs">
            You have no active API tokens.
          </Text>
        )}
      </SettingsSubsection>
    </SettingsSection>
  );
}

function formatDate(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? value
    : date.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}
