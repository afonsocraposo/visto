import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Alert, AppShell, Button, Group, Select, Tabs, Text, Title } from "@mantine/core";
import { IconCalendar, IconCompass, IconHome, IconLibrary, IconLogout, IconSearch } from "@tabler/icons-react";
import { WatchNow, WatchCalendar } from "../watch/Watch";
import { SearchPanel } from "../search/SearchPanel";
import { FeedPanel } from "../feed/FeedPanel";
import { LibraryArea } from "../library/LibraryArea";
import type { Tab, Theme, User } from "../../types";
import { connectionUnavailableEvent } from "../../lib/api";

export function Dashboard({ user, theme, setTheme }: { user: User; theme: Theme; setTheme: (theme: Theme) => void }) {
  const queryClient = useQueryClient();
  const [tab, setTab] = useState<Tab>("watch");
  const [view, setView] = useState("now");
  const [online, setOnline] = useState(() => navigator.onLine);
  const [checkingConnection, setCheckingConnection] = useState(false);
  const logout = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/v1/auth/logout", { method: "POST" });
      if (!response.ok) throw new Error("Could not sign out.");
    },
    onSuccess: () => {
      queryClient.clear();
      queryClient.setQueryData(["session"], null);
    },
  });

  const retryConnection = async () => {
    setCheckingConnection(true);
    try {
      const response = await fetch("/health", { cache: "no-store" });
      if (!response.ok) throw new Error("Server is not ready.");
      setOnline(true);
      await queryClient.invalidateQueries({ queryKey: ["user", user.id] });
    } catch {
      setOnline(false);
    } finally {
      setCheckingConnection(false);
    }
  };

  useEffect(() => {
    const onlineHandler = () => void retryConnection();
    const offlineHandler = () => setOnline(false);
    const unavailableHandler = () => setOnline(false);
    window.addEventListener("online", onlineHandler);
    window.addEventListener("offline", offlineHandler);
    window.addEventListener(connectionUnavailableEvent, unavailableHandler);
    return () => {
      window.removeEventListener("online", onlineHandler);
      window.removeEventListener("offline", offlineHandler);
      window.removeEventListener(connectionUnavailableEvent, unavailableHandler);
    };
  }, [user.id]);

  const nav = (value: Tab, label: string, Icon: typeof IconHome) => (
    <Button variant={tab === value ? "light" : "subtle"} leftSection={<Icon size={18} />} onClick={() => setTab(value)}>
      {label}
    </Button>
  );

  return (
    <AppShell header={{ height: 64 }} footer={{ height: 70 }} padding="md">
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between">
          <Title order={2}>Visto</Title>
          <Group>
            <Text size="sm">Hi, {user.display_name}</Text>
            <Button size="xs" variant="subtle" leftSection={<IconLogout size={16} />} loading={logout.isPending} onClick={() => logout.mutate()}>
              Sign out
            </Button>
            <Select
              aria-label="Color theme"
              value={theme}
              onChange={value => setTheme((value || "system") as Theme)}
              data={[
                { value: "system", label: "System" },
                { value: "light", label: "Light" },
                { value: "dark", label: "Dark" },
              ]}
              w={110}
            />
          </Group>
        </Group>
      </AppShell.Header>
      <AppShell.Main>
        {!online && (
          <Alert color="yellow" mb="md">
            <Group justify="space-between" align="center">
              <Text size="sm">You are offline. Saved information may be out of date, and changes need a connection.</Text>
              <Button size="xs" variant="default" loading={checkingConnection} onClick={() => void retryConnection()}>
                Retry connection
              </Button>
            </Group>
          </Alert>
        )}
        {logout.isError && <Alert color="red" mb="md">{logout.error.message}</Alert>}
        {tab === "watch" && (
          <>
            <Tabs value={view} onChange={value => setView(value || "now")}>
              <Tabs.List>
                <Tabs.Tab value="now">Now</Tabs.Tab>
                <Tabs.Tab value="calendar" leftSection={<IconCalendar size={16} />}>Calendar</Tabs.Tab>
              </Tabs.List>
            </Tabs>
            {view === "now" ? <WatchNow /> : <WatchCalendar />}
          </>
        )}
        {tab === "search" && <SearchPanel />}
        {tab === "feed" && <FeedPanel />}
        {tab === "library" && <LibraryArea user={user} />}
      </AppShell.Main>
      <AppShell.Footer>
        <Group justify="space-around" h="100%">
          {nav("watch", "Watch", IconHome)}
          {nav("search", "Search", IconSearch)}
          {nav("feed", "Feed", IconCompass)}
          {nav("library", "Library", IconLibrary)}
        </Group>
      </AppShell.Footer>
    </AppShell>
  );
}
