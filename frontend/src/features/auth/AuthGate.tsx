import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "@mantine/form";
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
import { backdropURL } from "../../lib/artwork";
import { api } from "../../lib/api";
import type { TrendingResponse, User } from "../../types";
import { pickLoginBackdrop, readLoginBackdrop, saveLoginBackdrop, tmdbTitleURL, type LoginBackdrop } from "./loginBackdrop";

function cachedBackdrop(): LoginBackdrop | null {
  try { return readLoginBackdrop(window.localStorage, Date.now()); } catch { return null; }
}

function cacheBackdrop(selection: LoginBackdrop): void {
  try { saveLoginBackdrop(window.localStorage, selection); } catch { /* Storage is optional. */ }
}

export function AuthGate() {
  const queryClient = useQueryClient();
  const [backdrop, setBackdrop] = useState<LoginBackdrop | null>(cachedBackdrop);
  const trending = useQuery({
    queryKey: ["public-trending", "week"],
    enabled: backdrop === null,
    queryFn: () => api.get<TrendingResponse>("/api/v1/public/trending?window=week", "Trending artwork is unavailable."),
    staleTime: 5 * 60_000,
  });
  useEffect(() => {
    if (backdrop || !trending.data) return;
    const cached = cachedBackdrop();
    const selected = cached ?? pickLoginBackdrop(trending.data, Date.now());
    if (!selected) return;
    if (!cached) cacheBackdrop(selected);
    setBackdrop(selected);
  }, [backdrop, trending.data]);
  useEffect(() => {
    if (!backdrop) return;
    const timeout = window.setTimeout(() => setBackdrop(null), Math.max(0, backdrop.expiresAt - Date.now()));
    return () => window.clearTimeout(timeout);
  }, [backdrop]);
  const setup = useQuery({
    queryKey: ["auth-status"],
    queryFn: async () => {
      const response = await fetch("/api/v1/auth/status");
      if (!response.ok) throw new Error("Could not check instance setup.");
      return response.json() as Promise<{ bootstrap_available: boolean }>;
    },
  });
  const [error, setError] = useState("");
  const form = useForm({ initialValues: { username: "", displayName: "", password: "" } });

  const signIn = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username: form.values.username, password: form.values.password }),
      });
      if (!response.ok) throw new Error("Invalid username or password.");
      return response.json() as Promise<User>;
    },
    onSuccess: user => queryClient.setQueryData(["session"], user),
  });

  const createAdmin = useMutation({
    mutationFn: async () => {
      const body = JSON.stringify({ username: form.values.username, display_name: form.values.displayName, password: form.values.password });
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
        body: JSON.stringify({ username: form.values.username, password: form.values.password }),
      });
      if (!login.ok) throw new Error("Administrator created. Please sign in.");
      return login.json() as Promise<User>;
    },
    onSuccess: user => queryClient.setQueryData(["session"], user),
  });

  const isFirstRun = setup.data?.bootstrap_available;
  const submit = async () => {
    setError("");
    try {
      await (isFirstRun ? createAdmin : signIn).mutateAsync();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not complete sign in.");
    }
  };
  const pending = isFirstRun ? createAdmin.isPending : signIn.isPending;

  return (
    <div className="auth-screen">
      <div className="auth-backdrop" style={backdrop ? { backgroundImage: `url(${backdropURL(backdrop.backdrop_path, "w1280")})` } : undefined} aria-hidden="true" />
      <div className="auth-screen-inner"><Paper className="auth-card" withBorder radius="xl" p="xl">
      <div className="auth-mark" aria-hidden="true">V</div>
      <Title order={1}>{isFirstRun ? "Set up Visto" : "Welcome to Visto"}</Title>
      <Text c="dimmed" mt="xs">
        {isFirstRun ? "Create the first administrator for this instance." : "Sign in to track what you watch."}
      </Text>
      {setup.isPending && <Group justify="center" py="xl"><Loader /></Group>}
      {setup.isError && <Alert color="red" mt="lg">Could not check instance setup. Refresh the page and try again.</Alert>}
      {!setup.isPending && !setup.isError &&
      <form onSubmit={form.onSubmit(() => void submit())}>
        <TextInput required minLength={3} maxLength={32} label="Username" mt="lg" {...form.getInputProps("username")} />
        {isFirstRun && <TextInput required maxLength={80} label="Your name" mt="md" {...form.getInputProps("displayName")} />}
        <PasswordInput required minLength={isFirstRun ? 12 : undefined} label="Password" mt="md" {...form.getInputProps("password")} />
        {error && <Alert color="red" mt="md">{error}</Alert>}
        <Button type="submit" loading={pending} fullWidth mt="lg">
          {isFirstRun ? "Create administrator" : "Sign in"}
        </Button>
      </form>}
      </Paper></div>
      {backdrop && <div className="auth-feature-caption"><Text size="xs" fw={700}>Trending on TMDB</Text><Text component="a" className="auth-feature-title" fw={650} href={tmdbTitleURL(backdrop)} target="_blank" rel="noopener noreferrer" aria-label={`View ${backdrop.title} on TMDB`}>{backdrop.title}</Text></div>}
    </div>
  );
}
