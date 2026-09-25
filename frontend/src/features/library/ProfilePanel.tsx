import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "@mantine/form";
import { Alert, Button, Group, Paper, PasswordInput, Select, Text, TextInput, Title } from "@mantine/core";
import { IconDownload } from "@tabler/icons-react";
import { useUserQueryKey } from "../auth/SessionContext";
import { PersonalTokensPanel } from "./PersonalTokensPanel";
import { ConnectedAppsPanel } from "./ConnectedAppsPanel";
import type { User } from "../../types";

export function ProfilePanel({ user }: { user: User }) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const form = useForm({ initialValues: { visibility: "private", timezone: "UTC", pushoverAppToken: "", pushoverUserKey: "" } });
  const settings = useQuery({
    queryKey: userQueryKey("profile-settings"),
    queryFn: async () => {
      const response = await fetch("/api/v1/profile/activity-settings");
      if (!response.ok) throw new Error();
      return response.json() as Promise<{ activity_visibility: string; timezone: string; pushover_available: boolean; pushover_enabled: boolean; has_pushover_app_token: boolean; has_pushover_user_key: boolean }>;
    },
  });
  useEffect(() => {
    if (settings.data) {
      form.setValues({ visibility: settings.data.activity_visibility, timezone: settings.data.timezone });
    }
  }, [settings.data]);
  const save = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/profile/activity-settings", {
        method: "PATCH", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ activity_visibility: form.values.visibility, timezone: form.values.timezone }),
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
        body: JSON.stringify({ enabled: settings.data?.pushover_enabled ?? false, app_token: form.values.pushoverAppToken, user_key: form.values.pushoverUserKey }),
      });
      if (!response.ok) {
        const result = await response.json().catch(() => ({}));
        throw new Error(result.error || "Could not save Pushover settings.");
      }
    }, onSuccess: async () => {
      form.setValues({ pushoverAppToken: "", pushoverUserKey: "" });
      await queryClient.invalidateQueries({ queryKey: userQueryKey("profile-settings") });
    },
  });
  const updatePushover = useMutation({
    mutationFn: async (enabled: boolean) => {
      const response = await fetch("/api/v1/profile/pushover-settings", {
        method: "PATCH", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled, app_token: form.values.pushoverAppToken, user_key: form.values.pushoverUserKey }),
      });
      if (!response.ok) {
        const result = await response.json().catch(() => ({}));
        throw new Error(result.error || "Could not update notification settings.");
      }
    }, onSuccess: async () => {
      form.setValues({ pushoverAppToken: "", pushoverUserKey: "" });
      await queryClient.invalidateQueries({ queryKey: userQueryKey("profile-settings") });
    },
  });
  const removePushoverKey = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/profile/pushover-credentials", { method: "DELETE" });
      if (!response.ok) throw new Error("Could not remove Pushover credentials.");
    }, onSuccess: async () => queryClient.invalidateQueries({ queryKey: userQueryKey("profile-settings") }),
  });
  const settingsUnavailable = settings.isPending || settings.isError;

  return <>
    <Paper withBorder p="md" mt="lg">
      <Title order={2}>Profile</Title>
      <Text size="sm" c="dimmed" mt="xs">Choose who can see your activity and which time zone the calendar uses.</Text>
      {settings.isError && <Alert color="red" mt="md">Profile settings are temporarily unavailable.</Alert>}
      <Select mt="md" label="Activity feed" disabled={settingsUnavailable} {...form.getInputProps("visibility")} data={[
        { value: "private", label: "Private" }, { value: "instance", label: "Visible to this instance" },
      ]} />
      <TextInput mt="md" label="Time zone" description="Use a time zone such as Europe/Lisbon or America/New_York." disabled={settingsUnavailable} {...form.getInputProps("timezone")} />
      {save.isError && <Alert color="red" mt="md">{save.error.message}</Alert>}
      <Button mt="md" disabled={settingsUnavailable} loading={save.isPending} onClick={() => save.mutate()}>Save settings</Button>

      <Title order={3} mt="xl">New episode alerts</Title>
      {settings.data?.pushover_available ? <>
        <Text size="sm" c="dimmed" mt="xs">Get a Pushover alert when an unwatched regular episode airs for a show you are watching.</Text>
        <PasswordInput mt="md" label={settings.data.has_pushover_app_token ? "Replace your Pushover application token" : "Your Pushover application token"} description="Create an application in your Pushover account to get this token." autoComplete="off" {...form.getInputProps("pushoverAppToken")} />
        <PasswordInput mt="md" label={settings.data.has_pushover_user_key ? "Replace your Pushover user key" : "Your Pushover user key"} autoComplete="off" {...form.getInputProps("pushoverUserKey")} />
        <Text size="xs" c="dimmed" mt={5}>Both credentials are encrypted before saving and are never shown again.</Text>
        {(savePushover.isError || updatePushover.isError || removePushoverKey.isError) && <Alert color="red" mt="md">{savePushover.error?.message || updatePushover.error?.message || removePushoverKey.error?.message}</Alert>}
        <Group mt="md">
          {(form.values.pushoverAppToken || form.values.pushoverUserKey) && <Button loading={savePushover.isPending} onClick={() => savePushover.mutate()}>Save credentials</Button>}
          {settings.data.pushover_enabled || (settings.data.has_pushover_app_token && settings.data.has_pushover_user_key) ? <Button variant="default" loading={updatePushover.isPending} onClick={() => updatePushover.mutate(!settings.data!.pushover_enabled)}>{settings.data.pushover_enabled ? "Turn alerts off" : "Turn alerts on"}</Button> : null}
          {(settings.data.has_pushover_app_token || settings.data.has_pushover_user_key) && <Button color="red" variant="subtle" loading={removePushoverKey.isPending} onClick={() => removePushoverKey.mutate()}>Remove credentials</Button>}
        </Group>
      </> : <Text size="sm" c="dimmed" mt="xs">This instance must enable encrypted storage for users’ notification credentials before Pushover can be used. Ask the administrator to configure VISTO_SECRET_ENCRYPTION_KEY.</Text>}

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
    <ConnectedAppsPanel />
    {user.role === "admin" && <CreateUserPanel />}
  </>;
}

function CreateUserPanel() {
  const [created, setCreated] = useState("");
  const form = useForm({ initialValues: { username: "", displayName: "", password: "" } });
  const create = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/users", {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username: form.values.username, display_name: form.values.displayName, password: form.values.password }),
      });
      if (!response.ok) {
        const result = await response.json().catch(() => ({}));
        throw new Error(result.error || "Could not create this account.");
      }
      return response.json() as Promise<User>;
    }, onSuccess: user => {
      setCreated(user.display_name); form.reset();
    },
  });

  return <Paper withBorder p="md" mt="lg">
    <Title order={2}>Add a family member</Title>
    <Text size="sm" c="dimmed" mt="xs">Each person gets a separate library and watch history.</Text>
    <form onSubmit={form.onSubmit(() => { setCreated(""); create.mutate(); })}>
      <TextInput required minLength={3} maxLength={32} label="Username" mt="md" {...form.getInputProps("username")} />
      <TextInput required maxLength={80} label="Name" mt="md" {...form.getInputProps("displayName")} />
      <PasswordInput required minLength={12} label="Temporary password" mt="md" {...form.getInputProps("password")} />
      {create.isError && <Alert color="red" mt="md">{create.error.message}</Alert>}
      {created && <Alert color="green" mt="md">Account created for {created}.</Alert>}
      <Button type="submit" loading={create.isPending} mt="md">Create account</Button>
    </form>
  </Paper>;
}
