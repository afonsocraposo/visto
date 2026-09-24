import { lazy, Suspense, useEffect, useState, type ReactNode } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useRouterState } from "@tanstack/react-router";
import { Alert, AppShell, Button, Center, Group, Loader, Select, Tabs, Text, Title } from "@mantine/core";
import { IconCalendar, IconCompass, IconHome, IconLogout, IconSearch, IconUserCircle } from "@tabler/icons-react";
import { WatchNow, WatchCalendar } from "../watch/Watch";
import { activeTabForLocation } from "./activeTab";
import type { LibraryStatus, MediaDetailTarget, Tab, Theme, User } from "../../types";
import { connectionUnavailableEvent } from "../../lib/api";

const SearchPanel = lazy(async () => ({ default: (await import("../search/SearchPanel")).SearchPanel }));
const FeedArea = lazy(async () => ({ default: (await import("../feed/FeedArea")).FeedArea }));
const LibraryArea = lazy(async () => ({ default: (await import("../library/LibraryArea")).LibraryArea }));
const LibraryListPage = lazy(async () => ({ default: (await import("../library/LibraryListPage")).LibraryListPage }));
const MediaDetailPage = lazy(async () => ({ default: (await import("../details/MediaDetailPage")).MediaDetailPage }));
const PersonDetailPage = lazy(async () => ({ default: (await import("../people/PersonDetailPage")).PersonDetailPage }));

export function Dashboard({ user, theme, setTheme }: { user: User; theme: Theme; setTheme: (theme: Theme) => void }) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const pathname = useRouterState({ select: state => state.location.pathname });
  const currentHref = useRouterState({ select: state => state.location.href });
  const detailSearch = new URLSearchParams(window.location.search);
  const returnTo = detailSearch.get("from");
  const tab: Tab = activeTabForLocation(pathname, window.location.search, window.location.origin);
  const detailMatch = pathname.match(/^\/media\/(movie|tv)\/(\d+)$/);
  const personMatch = pathname.match(/^\/people\/(\d+)$/);
  const personID = personMatch ? Number(personMatch[1]) : undefined;
  const listMatch = pathname.match(/^\/profile\/library\/(watching|completed|watchlist|paused|dropped)$/);
  const listStatus = listMatch?.[1] as LibraryStatus | undefined;
  const seedTitle = detailSearch.get("title");
  const detail: MediaDetailTarget | null = detailMatch ? {
    mediaType: detailMatch[1] as "movie" | "tv",
    tmdbID: Number(detailMatch[2]),
    mediaID: detailSearch.get("media") || undefined,
    episodeID: detailSearch.get("episode") || undefined,
    seasonNumber: detailSearch.get("season") ? Number(detailSearch.get("season")) : undefined,
    seed: seedTitle ? {
      tmdb_id: Number(detailMatch[2]),
      type: (detailSearch.get("type") as "movie" | "tv") || (detailMatch[1] as "movie" | "tv"),
      title: seedTitle,
      original_title: detailSearch.get("original_title") || seedTitle,
      overview: detailSearch.get("overview") || "",
      release_date: detailSearch.get("release_date") || "",
      poster_path: detailSearch.get("poster_path") || "",
      original_language: detailSearch.get("original_language") || "",
    } : undefined,
  } : null;
  const openDetail = (target: MediaDetailTarget) => void navigate({
    to: `/media/${target.mediaType}/${target.tmdbID}`,
    search: {
      from: currentHref,
      ...(target.mediaID ? { media: target.mediaID } : {}),
      ...(target.episodeID ? { episode: target.episodeID } : {}),
      ...(target.seasonNumber !== undefined ? { season: target.seasonNumber } : {}),
      ...(target.seed ? {
        title: target.seed.title,
        original_title: target.seed.original_title,
        overview: target.seed.overview,
        release_date: target.seed.release_date,
        poster_path: target.seed.poster_path,
        original_language: target.seed.original_language,
        type: target.seed.type,
      } : {}),
    },
  });
  const openPerson = (tmdbID: number) => void navigate({ to: `/people/${tmdbID}`, search: { from: currentHref } });
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
    <Button className="bottom-nav-button" variant={tab === value ? "light" : "subtle"} leftSection={<Icon size={18} stroke={1.8} />} onClick={() => void navigate({ to: value === "search" ? "/discover" : value === "library" ? "/profile" : `/${value}` })}>
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
        {personID ? <Deferred><PersonDetailPage personID={personID} onBack={() => void navigate({ to: safeReturnPath(returnTo, "/discover") })} onOpenDetail={openDetail} /></Deferred> : detail ? <Deferred><MediaDetailPage target={detail} onBack={() => void navigate({ to: safeReturnPath(returnTo, detail.mediaType === "tv" ? "/profile" : "/discover") })} onOpenDetail={openDetail} onOpenPerson={openPerson} /></Deferred> : listStatus ? <Deferred><LibraryListPage status={listStatus} onBack={() => void navigate({ to: "/profile" })} onOpenDetail={openDetail} /></Deferred> : tab === "watch" && (
          <>
            <Tabs className="watch-tabs" value={view} onChange={value => setView(value || "now")}>
              <Tabs.List>
                <Tabs.Tab value="now">To watch</Tabs.Tab>
                <Tabs.Tab value="calendar" leftSection={<IconCalendar size={16} />}>Calendar</Tabs.Tab>
              </Tabs.List>
            </Tabs>
            {view === "now" ? <WatchNow onOpenDetail={openDetail} /> : <WatchCalendar />}
          </>
        )}
        {!detail && !personID && tab === "search" && <Deferred><SearchPanel onOpenDetail={openDetail} /></Deferred>}
        {!detail && !personID && tab === "feed" && <Deferred><FeedArea /></Deferred>}
        {!detail && !personID && !listStatus && tab === "library" && <Deferred><LibraryArea user={user} onOpenDetail={openDetail} onOpenList={status => void navigate({ to: `/profile/library/${status}` })} /></Deferred>}
      </AppShell.Main>
      <AppShell.Footer className="visto-footer">
        <Group className="bottom-nav" justify="space-around" h="100%">
          {nav("watch", "Watching", IconHome)}
          {nav("search", "Discover", IconSearch)}
          {nav("feed", "Feed", IconCompass)}
          {nav("library", "Profile", IconUserCircle)}
        </Group>
      </AppShell.Footer>
    </AppShell>
  );
}

function Deferred({ children }: { children: ReactNode }) {
  return <Suspense fallback={<Center py="xl"><Loader size="sm" /></Center>}>{children}</Suspense>;
}

function safeReturnPath(returnTo: string | null, fallback: string): string {
  if (!returnTo) return fallback;
  try {
    const url = new URL(returnTo, window.location.origin);
    return url.origin === window.location.origin ? `${url.pathname}${url.search}${url.hash}` : fallback;
  } catch {
    return fallback;
  }
}
