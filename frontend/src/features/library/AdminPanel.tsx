import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "@mantine/form";
import { Alert, Button, Group, Paper, PasswordInput, Stack, Text, TextInput, Title } from "@mantine/core";
import { useUserQueryKey } from "../auth/SessionContext";
import type { User } from "../../types";

export function AdminPanel({ currentUser }: { currentUser: User }) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const users = useQuery({
    queryKey: userQueryKey("admin-users"),
    queryFn: async () => {
      const response = await fetch("/api/v1/users");
      if (!response.ok) throw new Error("Could not load users.");
      return response.json() as Promise<User[]>;
    },
  });
  const form = useForm({ initialValues: { email: "", name: "", password: "" } });
  const createUser = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/users", {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: form.values.email, name: form.values.name, password: form.values.password }),
      });
      if (!response.ok) throw new Error((await response.json().catch(() => ({}))).error || "Could not create account.");
      return response.json() as Promise<User>;
    },
    onSuccess: async () => { form.reset(); await queryClient.invalidateQueries({ queryKey: userQueryKey("admin-users") }); },
  });

  return <Stack>
    <Paper withBorder p="md">
      <Title order={2}>Manage users</Title>
      <Text size="sm" c="dimmed" mt="xs">Create and maintain accounts for this Visto instance.</Text>
      {users.isError && <Alert color="red" mt="md">{users.error.message}</Alert>}
      {users.data && <Stack mt="md">
        {users.data.map(account => <AdminUserRow key={account.id} account={account} currentUser={currentUser} />)}
      </Stack>}
    </Paper>

    <Paper withBorder p="md">
      <Title order={3}>Add a user</Title>
      <Text size="sm" c="dimmed" mt="xs">Each person gets a separate library and watch history.</Text>
      <form onSubmit={form.onSubmit(() => createUser.mutate())}>
        <TextInput required type="email" label="Email" mt="md" {...form.getInputProps("email")} />
        <TextInput required maxLength={80} label="Name" mt="md" {...form.getInputProps("name")} />
        <PasswordInput required minLength={12} label="Initial password" mt="md" {...form.getInputProps("password")} />
        {createUser.isError && <Alert color="red" mt="md">{createUser.error.message}</Alert>}
        <Button type="submit" loading={createUser.isPending} mt="md">Create account</Button>
      </form>
    </Paper>
  </Stack>;
}

function AdminUserRow({ account, currentUser }: { account: User; currentUser: User }) {
  const queryClient = useQueryClient();
  const userQueryKey = useUserQueryKey();
  const form = useForm({ initialValues: { displayName: account.name, password: "" } });
  const update = useMutation({
    mutationFn: async () => {
      const response = await fetch(`/api/v1/users/${encodeURIComponent(account.id)}`, {
        method: "PATCH", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: form.values.displayName, password: form.values.password }),
      });
      if (!response.ok) throw new Error((await response.json().catch(() => ({}))).error || "Could not update account.");
    },
    onSuccess: async () => {
      form.setFieldValue("password", "");
      setSaved(true);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: userQueryKey("admin-users") }),
        ...(account.id === currentUser.id ? [queryClient.invalidateQueries({ queryKey: ["session"] })] : []),
      ]);
    },
  });
  const remove = useMutation({
    mutationFn: async () => {
      const response = await fetch(`/api/v1/users/${encodeURIComponent(account.id)}`, { method: "DELETE" });
      if (!response.ok) throw new Error((await response.json().catch(() => ({}))).error || "Could not delete account.");
    },
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: userQueryKey("admin-users") }),
  });
  const [saved, setSaved] = useState(false);

  return <Paper withBorder p="md" radius="md">
    <Group justify="space-between" align="start">
      <div><Text fw={600}>{account.name}{account.id === currentUser.id ? " (you)" : ""}</Text><Text size="sm" c="dimmed">{account.email} · {account.role === "admin" ? "Admin" : "User"}</Text></div>
      {account.id !== currentUser.id && <Button color="red" variant="subtle" loading={remove.isPending} onClick={() => {
        if (window.confirm(`Delete ${account.name}'s account and personal data? This cannot be undone.`)) remove.mutate();
      }}>Delete</Button>}
    </Group>
    <form onSubmit={form.onSubmit(() => { setSaved(false); update.mutate(); })}>
      <TextInput label="Name" mt="md" required maxLength={80} {...form.getInputProps("displayName")} />
      <PasswordInput label="New password" description="Leave blank to keep the current password." mt="md" minLength={12} {...form.getInputProps("password")} />
      {(update.isError || remove.isError) && <Alert color="red" mt="md">{update.error?.message || remove.error?.message}</Alert>}
      {saved && <Alert color="green" mt="md">Account updated.</Alert>}
      <Button type="submit" variant="default" mt="md" loading={update.isPending}>Save changes</Button>
    </form>
  </Paper>;
}
