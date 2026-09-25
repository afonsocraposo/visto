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
import {
  pickLoginBackdrop,
  readLoginBackdrop,
  saveLoginBackdrop,
  tmdbTitleURL,
  type LoginBackdrop,
} from "./loginBackdrop";

function cachedBackdrop(): LoginBackdrop | null {
  try {
    return readLoginBackdrop(window.localStorage, Date.now());
  } catch {
    return null;
  }
}

function cacheBackdrop(selection: LoginBackdrop): void {
  try {
    saveLoginBackdrop(window.localStorage, selection);
  } catch {
    /* Storage is optional. */
  }
}

function GoogleIcon() {
  return (
    <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24">
      <path
        fill="#4285F4"
        d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.22 3.31v2.77h3.58c2.1-1.93 3.28-4.78 3.28-8.09Z"
      />
      <path
        fill="#34A853"
        d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.58-2.77c-.98.66-2.23 1.06-3.7 1.06-2.85 0-5.27-1.92-6.13-4.5H2.18v2.84A11 11 0 0 0 12 23Z"
      />
      <path
        fill="#FBBC05"
        d="M5.87 14.13a6.6 6.6 0 0 1 0-4.2V7.09H2.18a11 11 0 0 0 0 9.82l3.69-2.78Z"
      />
      <path
        fill="#EA4335"
        d="M12 4.5c1.62 0 3.06.56 4.21 1.64l3.15-3.15A10.56 10.56 0 0 0 12 0a11 11 0 0 0-9.82 7.09l3.69 2.84C6.73 7.42 9.15 4.5 12 4.5Z"
      />
    </svg>
  );
}

export function AuthGate() {
  const queryClient = useQueryClient();
  const [mode, setMode] = useState<"login" | "signup">("login");
  const [backdrop, setBackdrop] = useState<LoginBackdrop | null>(cachedBackdrop);
  const trending = useQuery({
    queryKey: ["public-trending", "week"],
    enabled: backdrop === null,
    queryFn: () =>
      api.get<TrendingResponse>(
        "/api/v1/public/trending?window=week",
        "Trending artwork is unavailable.",
      ),
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
    const timeout = window.setTimeout(
      () => setBackdrop(null),
      Math.max(0, backdrop.expiresAt - Date.now()),
    );
    return () => window.clearTimeout(timeout);
  }, [backdrop]);
  const setup = useQuery({
    queryKey: ["auth-status"],
    queryFn: async () => {
      const response = await fetch("/api/v1/auth/status");
      if (!response.ok) throw new Error("Could not check instance setup.");
      return response.json() as Promise<{
        bootstrap_available: boolean;
        signup_enabled: boolean;
        google_enabled: boolean;
      }>;
    },
  });
  const [error, setError] = useState(() => {
    const params = new URLSearchParams(window.location.search);
    if (params.get("auth_error") !== "google") return "";
    return params.get("reason") === "signup_disabled"
      ? "Google sign-in is available, but this instance does not allow new accounts."
      : "Google sign-in could not be completed. Try again or use your email and password.";
  });
  const form = useForm({ initialValues: { email: "", name: "", password: "" } });

  const signIn = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: form.values.email, password: form.values.password }),
      });
      if (!response.ok) throw new Error("Invalid email or password.");
      return response.json() as Promise<User>;
    },
    onSuccess: (user) => queryClient.setQueryData(["session"], user),
  });

  const createAdmin = useMutation({
    mutationFn: async () => {
      const body = JSON.stringify({
        email: form.values.email,
        name: form.values.name,
        password: form.values.password,
      });
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
        body: JSON.stringify({ email: form.values.email, password: form.values.password }),
      });
      if (!login.ok) throw new Error("Administrator created. Please sign in.");
      return login.json() as Promise<User>;
    },
    onSuccess: async (user) => {
      queryClient.setQueryData(["session"], user);
      await queryClient.invalidateQueries({ queryKey: ["auth-status"] });
    },
  });

  const signUp = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/auth/signup", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email: form.values.email,
          name: form.values.name,
          password: form.values.password,
        }),
      });
      if (!response.ok) {
        const result = await response.json().catch(() => ({}));
        throw new Error(result.error || "Could not create your account.");
      }
      return response.json() as Promise<User>;
    },
    onSuccess: (user) => queryClient.setQueryData(["session"], user),
  });

  const isFirstRun = setup.data?.bootstrap_available;
  const isSignup = !isFirstRun && mode === "signup" && setup.data?.signup_enabled;
  const submit = async () => {
    setError("");
    try {
      await (isFirstRun ? createAdmin : isSignup ? signUp : signIn).mutateAsync();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not complete sign in.");
    }
  };
  const pending = isFirstRun
    ? createAdmin.isPending
    : isSignup
      ? signUp.isPending
      : signIn.isPending;

  return (
    <div className="auth-screen">
      <div
        className="auth-backdrop"
        style={
          backdrop
            ? { backgroundImage: `url(${backdropURL(backdrop.backdrop_path, "w1280")})` }
            : undefined
        }
        aria-hidden="true"
      />
      <div className="auth-screen-inner">
        <Paper className="auth-card" withBorder radius="xl" p="xl">
          <img className="auth-mark" src="/icon.svg?v=2" alt="" aria-hidden="true" />
          <Title order={1}>
            {isFirstRun ? "Set up Visto" : isSignup ? "Create your account" : "Welcome to Visto"}
          </Title>
          <Text c="dimmed" mt="xs">
            {isFirstRun
              ? "Create the first administrator for this instance."
              : isSignup
                ? "Create an account to start tracking what you watch."
                : "Sign in to track what you watch."}
          </Text>
          {setup.isPending && (
            <Group justify="center" py="xl">
              <Loader />
            </Group>
          )}
          {setup.isError && (
            <Alert color="red" mt="lg">
              Could not check instance setup. Refresh the page and try again.
            </Alert>
          )}
          {!setup.isPending && !setup.isError && (
            <form onSubmit={form.onSubmit(() => void submit())}>
              {(isFirstRun || isSignup) && (
                <TextInput
                  required
                  maxLength={80}
                  label="Your name"
                  mt="lg"
                  {...form.getInputProps("name")}
                />
              )}
              <TextInput
                required
                type="email"
                autoComplete="email"
                label="Email"
                mt="lg"
                {...form.getInputProps("email")}
              />
              <PasswordInput
                required
                minLength={isFirstRun || isSignup ? 12 : undefined}
                label="Password"
                mt="md"
                {...form.getInputProps("password")}
              />
              {error && (
                <Alert color="red" mt="md">
                  {error}
                </Alert>
              )}
              <Button type="submit" loading={pending} fullWidth mt="lg">
                {isFirstRun ? "Create administrator" : isSignup ? "Create account" : "Sign in"}
              </Button>
            </form>
          )}
          {!setup.isPending && !setup.isError && !isFirstRun && setup.data?.google_enabled && (
            <Button
              component="a"
              href="/api/v1/auth/google"
              variant="default"
              fullWidth
              mt="md"
              leftSection={<GoogleIcon />}
            >
              Continue with Google
            </Button>
          )}
          {!setup.isPending && !setup.isError && !isFirstRun && setup.data?.signup_enabled && (
            <Button
              variant="subtle"
              fullWidth
              mt="xs"
              onClick={() => {
                setError("");
                setMode(isSignup ? "login" : "signup");
              }}
            >
              {isSignup ? "Already have an account? Sign in" : "New here? Create an account"}
            </Button>
          )}
        </Paper>
      </div>
      {backdrop && (
        <div className="auth-feature-caption">
          <Text size="xs" fw={700}>
            Trending on TMDB
          </Text>
          <Text
            component="a"
            className="auth-feature-title"
            fw={650}
            href={tmdbTitleURL(backdrop)}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={`View ${backdrop.title} on TMDB`}
          >
            {backdrop.title}
          </Text>
        </div>
      )}
    </div>
  );
}
