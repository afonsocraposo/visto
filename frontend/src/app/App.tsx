import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { createTheme, Group, Loader, MantineProvider } from "@mantine/core";
import { AuthGate } from "../features/auth/AuthGate";
import { SessionProvider } from "../features/auth/SessionContext";
import { forcedColorScheme } from "./theme";
import { router } from "./router";
import { RouterProvider } from "@tanstack/react-router";
import type { Theme, User } from "../types";

const themeDefinition = createTheme({
  primaryColor: "amber",
  primaryShade: { light: 7, dark: 5 },
  colors: {
    amber: ["#fff7e0", "#ffedbd", "#ffe090", "#ffd064", "#fac147", "#f2b544", "#d99420", "#b77316", "#925912", "#77480f"],
  },
  fontFamily: "Inter, ui-sans-serif, system-ui, sans-serif",
  headings: { fontFamily: "Inter, ui-sans-serif, system-ui, sans-serif", fontWeight: "750" },
  defaultRadius: "md",
});

export function App() {
  const session = useQuery({
    queryKey: ["session"],
    queryFn: async (): Promise<User | null> => {
      const response = await fetch("/api/v1/me");
      return response.ok ? response.json() : null;
    },
  });
  const [theme, setTheme] = useState<Theme>(() => (localStorage.getItem("visto-theme") as Theme) || "system");

  useEffect(() => {
    localStorage.setItem("visto-theme", theme);
  }, [theme]);

  return (
    <MantineProvider theme={themeDefinition} defaultColorScheme="auto" forceColorScheme={forcedColorScheme(theme)}>
      {session.isPending ? (
        <Group justify="center" mt="xl"><Loader /></Group>
      ) : session.data ? (
        <SessionProvider user={session.data}>
          <RouterProvider router={router} context={{ user: session.data, theme, setTheme }} />
        </SessionProvider>
      ) : (
        <AuthGate />
      )}
    </MantineProvider>
  );
}
