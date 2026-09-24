import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Group, Loader, MantineProvider } from "@mantine/core";
import { Dashboard } from "../features/navigation/Dashboard";
import { AuthGate } from "../features/auth/AuthGate";
import { SessionProvider } from "../features/auth/SessionContext";
import { forcedColorScheme } from "./theme";
import type { Theme, User } from "../types";

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
    <MantineProvider defaultColorScheme="auto" forceColorScheme={forcedColorScheme(theme)}>
      {session.isPending ? (
        <Group justify="center" mt="xl"><Loader /></Group>
      ) : session.data ? (
        <SessionProvider user={session.data}>
          <Dashboard user={session.data} theme={theme} setTheme={setTheme} />
        </SessionProvider>
      ) : (
        <AuthGate />
      )}
    </MantineProvider>
  );
}
