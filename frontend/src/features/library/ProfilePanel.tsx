import { useEffect, useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Group, Paper, PasswordInput, Select, Text, TextInput, Title } from "@mantine/core";
import { IconDownload } from "@tabler/icons-react";
import { useUserQueryKey } from "../auth/SessionContext";
import type { User } from "../../types";

export function ProfilePanel({ user }: { user: User }) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const [visibility, setVisibility] = useState("private");
  const [timezone, setTimezone] = useState("UTC");
  const settings = useQuery({
    queryKey: userQueryKey("profile-settings"),
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
        method: "PATCH", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ activity_visibility: visibility, timezone }),
      });
      if (!response.ok) throw new Error("Could not save settings.");
    }, onSuccess: () => queryClient.invalidateQueries({ queryKey: userQueryKey("profile-settings") }),
  });

  return <>
    <Paper withBorder p="md" mt="lg">
      <Title order={2}>Profile</Title>
      <Text size="sm" c="dimmed" mt="xs">Choose who can see your activity and which time zone the calendar uses.</Text>
      {settings.isError && <Alert color="red" mt="md">Profile settings are temporarily unavailable.</Alert>}
      <Select mt="md" label="Activity feed" value={visibility} onChange={value => setVisibility(value || "private")} data={[
        { value: "private", label: "Private" }, { value: "instance", label: "Visible to this instance" },
      ]} />
      <TextInput mt="md" label="Time zone" description="Use a time zone such as Europe/Lisbon or America/New_York." value={timezone} onChange={event => setTimezone(event.currentTarget.value)} />
      {save.isError && <Alert color="red" mt="md">{save.error.message}</Alert>}
      <Button mt="md" loading={save.isPending} onClick={() => save.mutate()}>Save settings</Button>

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
