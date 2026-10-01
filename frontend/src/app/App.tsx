import { lazy, Suspense, useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Alert, Button, createTheme, Group, Loader, MantineProvider, Stack } from "@mantine/core";
import { oauthReturnLocation } from "../features/auth/oauthReturn";
import { SessionProvider } from "../features/auth/SessionContext";
import { loadSession } from "../features/auth/session";
import { forcedColorScheme } from "./theme";
import { router } from "./router";
import { RouterProvider } from "@tanstack/react-router";
import { Notifications } from "@mantine/notifications";
import type { Theme } from "../types";

const AuthGate = lazy(async () => ({
  default: (await import("../features/auth/AuthGate")).AuthGate,
}));

// Local system fonts: fast and fitting for a self-hosted app; personality comes from layout.
const systemFont =
  'ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';

const themeDefinition = createTheme({
  primaryColor: "amber",
  primaryShade: { light: 7, dark: 5 },
  colors: {
    amber: [
      "#fff7e0",
      "#ffedbd",
      "#ffe090",
      "#ffd064",
      "#fac147",
      "#f2b544",
      "#d99420",
      "#b77316",
      "#925912",
      "#77480f",
    ],
  },
  fontFamily: systemFont,
  headings: { fontFamily: systemFont, fontWeight: "750" },
  // Small controls ~8px, cards ~12px, large surfaces ~18px, sheets/modals ~20px.
  radius: { xs: "6px", sm: "8px", md: "12px", lg: "18px", xl: "20px" },
  defaultRadius: "sm",
  shadows: {
    xs: "0 1px 2px rgb(23 34 48 / 0.05)",
    sm: "0 2px 8px rgb(23 34 48 / 0.06)",
    md: "0 10px 28px rgb(23 34 48 / 0.12)",
    lg: "0 18px 44px rgb(0 0 0 / 0.3)",
    xl: "0 24px 70px rgb(0 0 0 / 0.35)",
  },
  focusRing: "auto",
  cursorType: "pointer",
  components: {
    Modal: { defaultProps: { radius: "xl" } },
    Drawer: { defaultProps: { radius: "xl" } },
    Paper: { defaultProps: { radius: "md" } },
    Tooltip: {
      defaultProps: { openDelay: 300, events: { hover: true, focus: true, touch: false } },
    },
  },
  other: {
    success: "#48b7ae",
    motion: { fast: 100, normal: 160, slow: 220 },
    easeStandard: "cubic-bezier(.2, .8, .2, 1)",
  },
});

export function App() {
  const setup = useQuery({
    queryKey: ["auth-status"],
    queryFn: async () => {
      const response = await fetch("/api/v1/auth/status");
      if (!response.ok) throw new Error("Could not check instance setup.");
      return response.json() as Promise<{ bootstrap_available: boolean; signup_enabled: boolean }>;
    },
  });
  const session = useQuery({
    queryKey: ["session"],
    queryFn: loadSession,
  });
  const [theme, setTheme] = useState<Theme>(
    () => (localStorage.getItem("visto-theme") as Theme) || "system",
  );
  const oauthReturn = oauthReturnLocation(window.location.search, window.location.origin);

  useEffect(() => {
    localStorage.setItem("visto-theme", theme);
  }, [theme]);

  useEffect(() => {
    if (session.data && oauthReturn) window.location.replace(oauthReturn);
  }, [oauthReturn, session.data]);

  return (
    <MantineProvider
      theme={themeDefinition}
      defaultColorScheme="auto"
      forceColorScheme={forcedColorScheme(theme)}
    >
      <Notifications position="bottom-center" autoClose={5000} />
      {setup.isPending || session.isPending ? (
        <Group justify="center" mt="xl">
          <Loader />
        </Group>
      ) : setup.isError || (session.isError && !session.data) ? (
        <Stack align="center" mt="xl">
          <Alert color="red">Could not connect to Visto. Try again.</Alert>
          <Button onClick={() => void Promise.all([setup.refetch(), session.refetch()])}>
            Retry
          </Button>
        </Stack>
      ) : !setup.data?.bootstrap_available && session.data ? (
        <SessionProvider user={session.data}>
          <RouterProvider router={router} context={{ user: session.data, theme, setTheme }} />
        </SessionProvider>
      ) : (
        <Suspense fallback={<Loader />}>
          <AuthGate />
        </Suspense>
      )}
    </MantineProvider>
  );
}
