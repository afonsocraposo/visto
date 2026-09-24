import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Alert, AppShell, Button, Group, Select, Tabs, Text, Title } from "@mantine/core";
import { IconCalendar, IconCompass, IconHome, IconLogout, IconSearch, IconUserCircle } from "@tabler/icons-react";
import { WatchNow, WatchCalendar } from "../watch/Watch";
import { SearchPanel } from "../search/SearchPanel";
import { FeedArea } from "../feed/FeedArea";
import { LibraryArea } from "../library/LibraryArea";
import { MediaDetailPage } from "../details/MediaDetailPage";
import type { MediaDetailTarget, Tab, Theme, User } from "../../types";
import { connectionUnavailableEvent } from "../../lib/api";

export function Dashboard({ user, theme, setTheme }: { user: User; theme: Theme; setTheme: (theme: Theme) => void }) {
  const queryClient = useQueryClient();
  const [tab, setTab] = useState<Tab>("watch");
  const [view, setView] = useState("now");
  const [detail, setDetail] = useState<MediaDetailTarget | null>(null);
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
    <Button className="bottom-nav-button" variant={tab === value ? "light" : "subtle"} leftSection={<Icon size={18} stroke={1.8} />} onClick={() => setTab(value)}>
      {label}
    </Button>
  );

  return (
    <AppShell className="visto-shell" header={{ height: 72 }} footer={{ height: 76 }} padding={0}>
      <AppShell.Header className="visto-header">
        <Group className="visto-header-inner" h="100%" justify="space-between">
          <Group gap="sm">
            <div className="visto-mark" aria-hidden="true">V</div>
            <div>
              <Title className="visto-wordmark" order={2}>Visto</Title>
              <Text className="visto-subtitle" size="xs">Your watchroom</Text>
            </div>
          </Group>
          <Group gap="xs">
            <Text className="welcome-name" size="sm">Hi, {user.display_name}</Text>
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
              className="theme-select"
              w={108}
            />
          </Group>
        </Group>
      </AppShell.Header>
      <AppShell.Main className="visto-main">
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
        {detail ? <MediaDetailPage target={detail} onBack={() => setDetail(null)} /> : tab === "watch" && (
          <>
            <Tabs className="watch-tabs" value={view} onChange={value => setView(value || "now")}>
              <Tabs.List>
                <Tabs.Tab value="now">Now</Tabs.Tab>
                <Tabs.Tab value="calendar" leftSection={<IconCalendar size={16} />}>Calendar</Tabs.Tab>
              </Tabs.List>
            </Tabs>
            {view === "now" ? <WatchNow onOpenDetail={setDetail} /> : <WatchCalendar />}
          </>
        )}
        {!detail && tab === "search" && <SearchPanel onOpenDetail={setDetail} />}
        {!detail && tab === "feed" && <FeedArea />}
        {!detail && tab === "library" && <LibraryArea user={user} onOpenDetail={setDetail} />}
      </AppShell.Main>
      <AppShell.Footer className="visto-footer">
        <Group className="bottom-nav" justify="space-around" h="100%">
          {nav("watch", "Watch", IconHome)}
          {nav("search", "Discover", IconSearch)}
          {nav("feed", "Feed", IconCompass)}
          {nav("library", "Profile", IconUserCircle)}
        </Group>
      </AppShell.Footer>
    </AppShell>
  );
}
