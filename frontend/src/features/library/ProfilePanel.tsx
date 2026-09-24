import { useEffect, useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Group, Paper, PasswordInput, Select, Text, TextInput, Title } from "@mantine/core";
import { IconDownload } from "@tabler/icons-react";
import { useUserQueryKey } from "../auth/SessionContext";
import { PersonalTokensPanel } from "./PersonalTokensPanel";
import type { User } from "../../types";

export function ProfilePanel({ user }: { user: User }) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const [visibility, setVisibility] = useState("private");
  const [timezone, setTimezone] = useState("UTC");
  const [pushoverUserKey, setPushoverUserKey] = useState("");
  const settings = useQuery({
    queryKey: userQueryKey("profile-settings"),
    queryFn: async () => {
      const response = await fetch("/api/v1/profile/activity-settings");
      if (!response.ok) throw new Error();
      return response.json() as Promise<{ activity_visibility: string; timezone: string; pushover_available: boolean; pushover_enabled: boolean; has_pushover_key: boolean }>;
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
        method: "PATCH", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ activity_visibility: visibility, timezone }),
      });
      if (!response.ok) throw new Error("Could not save settings.");
    }, onSuccess: async () => Promise.all([
      queryClient.invalidateQueries({ queryKey: userQueryKey("profile-settings") }),
      queryClient.invalidateQueries({ queryKey: userQueryKey("feed") }),
    ]),
  });
  const savePushover = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/profile/pushover-settings", {
        method: "PATCH", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled: settings.data?.pushover_enabled ?? false, user_key: pushoverUserKey }),
      });
      if (!response.ok) {
        const result = await response.json().catch(() => ({}));
        throw new Error(result.error || "Could not save Pushover settings.");
      }
    }, onSuccess: async () => {
      setPushoverUserKey("");
      await queryClient.invalidateQueries({ queryKey: userQueryKey("profile-settings") });
    },
  });
  const updatePushover = useMutation({
    mutationFn: async (enabled: boolean) => {
      const response = await fetch("/api/v1/profile/pushover-settings", {
        method: "PATCH", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled, user_key: pushoverUserKey }),
      });
      if (!response.ok) {
        const result = await response.json().catch(() => ({}));
        throw new Error(result.error || "Could not update notification settings.");
      }
    }, onSuccess: async () => {
      setPushoverUserKey("");
      await queryClient.invalidateQueries({ queryKey: userQueryKey("profile-settings") });
    },
  });
  const removePushoverKey = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/profile/pushover-key", { method: "DELETE" });
      if (!response.ok) throw new Error("Could not remove the Pushover key.");
    }, onSuccess: async () => queryClient.invalidateQueries({ queryKey: userQueryKey("profile-settings") }),
  });
  const settingsUnavailable = settings.isPending || settings.isError;

  return <>
    <Paper withBorder p="md" mt="lg">
      <Title order={2}>Profile</Title>
      <Text size="sm" c="dimmed" mt="xs">Choose who can see your activity and which time zone the calendar uses.</Text>
      {settings.isError && <Alert color="red" mt="md">Profile settings are temporarily unavailable.</Alert>}
      <Select mt="md" label="Activity feed" value={visibility} disabled={settingsUnavailable} onChange={value => setVisibility(value || "private")} data={[
        { value: "private", label: "Private" }, { value: "instance", label: "Visible to this instance" },
      ]} />
      <TextInput mt="md" label="Time zone" description="Use a time zone such as Europe/Lisbon or America/New_York." value={timezone} disabled={settingsUnavailable} onChange={event => setTimezone(event.currentTarget.value)} />
      {save.isError && <Alert color="red" mt="md">{save.error.message}</Alert>}
      <Button mt="md" disabled={settingsUnavailable} loading={save.isPending} onClick={() => save.mutate()}>Save settings</Button>

      <Title order={3} mt="xl">New episode alerts</Title>
      {settings.data?.pushover_available ? <>
        <Text size="sm" c="dimmed" mt="xs">Get a Pushover alert when an unwatched regular episode airs for a show you are watching.</Text>
        <PasswordInput mt="md" label={settings.data.has_pushover_key ? "Replace Pushover user key" : "Pushover user key"} value={pushoverUserKey} onChange={event => setPushoverUserKey(event.currentTarget.value)} autoComplete="off" />
        <Text size="xs" c="dimmed" mt={5}>Your key is encrypted before it is saved and is never shown again.</Text>
        {(savePushover.isError || updatePushover.isError || removePushoverKey.isError) && <Alert color="red" mt="md">{savePushover.error?.message || updatePushover.error?.message || removePushoverKey.error?.message}</Alert>}
        <Group mt="md">
          {pushoverUserKey && <Button loading={savePushover.isPending} onClick={() => savePushover.mutate()}>Save key</Button>}
          {settings.data.has_pushover_key && <Button variant="default" loading={updatePushover.isPending} onClick={() => updatePushover.mutate(!settings.data!.pushover_enabled)}>{settings.data.pushover_enabled ? "Turn alerts off" : "Turn alerts on"}</Button>}
          {settings.data.has_pushover_key && <Button color="red" variant="subtle" loading={removePushoverKey.isPending} onClick={() => removePushoverKey.mutate()}>Remove key</Button>}
        </Group>
      </> : <Text size="sm" c="dimmed" mt="xs">Pushover alerts are not configured by this Visto instance.</Text>}

      <Title order={3} mt="xl">Export your data</Title>
      <Text size="sm" c="dimmed" mt="xs">These downloads include only your library, ratings, and watch history.</Text>
      <Group mt="md">
        <Button component="a" href="/api/v1/export/json" download="visto-export.json" variant="default" leftSection={<IconDownload size={16} />}>
          Download JSON
        </Button>
        <Button component="a" href="/api/v1/export/csv" download="visto-export.csv" variant="default" leftSection={<IconDownload size={16} />}>
          Download CSV
        </Button>
      </Group>
    </Paper>
    <PersonalTokensPanel />
    {user.role === "admin" && <CreateUserPanel />}
  </>;
}

function CreateUserPanel() {
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [created, setCreated] = useState("");
  const create = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/users", {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, display_name: displayName, password }),
      });
      if (!response.ok) {
        const result = await response.json().catch(() => ({}));
        throw new Error(result.error || "Could not create this account.");
      }
      return response.json() as Promise<User>;
    }, onSuccess: user => {
      setCreated(user.display_name); setUsername(""); setDisplayName(""); setPassword("");
    },
  });
  const submit = async (event: FormEvent) => {
    event.preventDefault(); setCreated(""); await create.mutateAsync();
  };

  return <Paper withBorder p="md" mt="lg">
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
  </Paper>;
}
