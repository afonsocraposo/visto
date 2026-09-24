import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Alert,
  Button,
  Group,
  Loader,
  Paper,
  PasswordInput,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import type { User } from "../../types";

export function AuthGate() {
  const queryClient = useQueryClient();
  const setup = useQuery({
    queryKey: ["auth-status"],
    queryFn: async () => {
      const response = await fetch("/api/v1/auth/status");
      if (!response.ok) throw new Error("Could not check instance setup.");
      return response.json() as Promise<{ bootstrap_available: boolean }>;
    },
  });
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  const signIn = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });
      if (!response.ok) throw new Error("Invalid username or password.");
      return response.json() as Promise<User>;
    },
    onSuccess: user => queryClient.setQueryData(["session"], user),
  });

  const createAdmin = useMutation({
    mutationFn: async () => {
      const body = JSON.stringify({ username, display_name: displayName, password });
      const bootstrap = await fetch("/api/v1/auth/bootstrap", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body,
      });
      if (!bootstrap.ok) {
        const result = await bootstrap.json().catch(() => ({}));
        throw new Error(result.error || "Could not create the administrator.");
      }
      const login = await fetch("/api/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });
      if (!login.ok) throw new Error("Administrator created. Please sign in.");
      return login.json() as Promise<User>;
    },
    onSuccess: user => queryClient.setQueryData(["session"], user),
  });

  if (setup.isPending) {
    return <Group justify="center" mt="xl"><Loader /></Group>;
  }

  const isFirstRun = setup.data?.bootstrap_available;
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setError("");
    try {
      await (isFirstRun ? createAdmin : signIn).mutateAsync();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not complete sign in.");
    }
  };
  const pending = isFirstRun ? createAdmin.isPending : signIn.isPending;

  return (
    <Paper withBorder radius="md" p="xl" maw={440} mx="auto" mt="xl">
      <Title order={1}>{isFirstRun ? "Set up Visto" : "Welcome to Visto"}</Title>
      <Text c="dimmed" mt="xs">
        {isFirstRun ? "Create the first administrator for this instance." : "Sign in to track what you watch."}
      </Text>
      <form onSubmit={event => void submit(event)}>
        <TextInput required minLength={3} maxLength={32} label="Username" value={username} onChange={event => setUsername(event.currentTarget.value)} mt="lg" />
        {isFirstRun && <TextInput required maxLength={80} label="Your name" value={displayName} onChange={event => setDisplayName(event.currentTarget.value)} mt="md" />}
        <PasswordInput required minLength={isFirstRun ? 12 : undefined} label="Password" value={password} onChange={event => setPassword(event.currentTarget.value)} mt="md" />
        {error && <Alert color="red" mt="md">{error}</Alert>}
        <Button type="submit" loading={pending} fullWidth mt="lg">
          {isFirstRun ? "Create administrator" : "Sign in"}
        </Button>
      </form>
    </Paper>
  );
}
