import { lazy, Suspense, useEffect, useState, type ReactNode } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useRouterState } from "@tanstack/react-router";
import { Alert, AppShell, Button, Center, Group, Loader, Tabs, Text } from "@mantine/core";
import {
  IconCalendar,
  IconCompass,
  IconHome,
  IconSearch,
  IconUserCircle,
} from "@tabler/icons-react";
import { WatchNow, WatchCalendar } from "../watch/Watch";
import type { LibraryStatus, MediaDetailTarget, Tab, Theme, User } from "../../types";
import { connectionUnavailableEvent } from "../../lib/api";
import { clearSignedInCache, endCurrentSession } from "../auth/logout";

const SearchPanel = lazy(async () => ({
  default: (await import("../search/SearchPanel")).SearchPanel,
}));
const FeedArea = lazy(async () => ({ default: (await import("../feed/FeedArea")).FeedArea }));
const LibraryArea = lazy(async () => ({
  default: (await import("../library/LibraryArea")).LibraryArea,
}));
const LibraryListPage = lazy(async () => ({
  default: (await import("../library/LibraryListPage")).LibraryListPage,
}));
const MediaDetailPage = lazy(async () => ({
  default: (await import("../details/MediaDetailPage")).MediaDetailPage,
}));
const PersonDetailPage = lazy(async () => ({
  default: (await import("../people/PersonDetailPage")).PersonDetailPage,
}));

export type DashboardPage =
  | { kind: "watch" | "discover" | "feed" | "profile" }
  | { kind: "library-list"; status: LibraryStatus }
  | { kind: "media"; target: MediaDetailTarget; returnTo?: string }
  | { kind: "person"; personID: number; returnTo?: string };

export function Dashboard({
  user,
  theme,
  setTheme,
  page,
}: {
  user: User;
  theme: Theme;
  setTheme: (theme: Theme) => void;
  page: DashboardPage;
}) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const currentHref = useRouterState({ select: (state) => state.location.href });
  const detail = page.kind === "media" ? page.target : null;
  const personID = page.kind === "person" ? page.personID : undefined;
  const listStatus = page.kind === "library-list" ? page.status : undefined;
  const returnTo = page.kind === "media" || page.kind === "person" ? page.returnTo : undefined;
  const tab: Tab =
    page.kind === "discover"
      ? "search"
      : page.kind === "feed"
        ? "feed"
        : page.kind === "profile" || page.kind === "library-list"
          ? "library"
          : "watch";
  const openDetail = (target: MediaDetailTarget) =>
    void navigate({
      to: `/media/${target.mediaType}/${target.tmdbID}`,
      search: {
        from: currentHref,
        ...(target.mediaID ? { media: target.mediaID } : {}),
        ...(target.episodeID ? { episode: target.episodeID } : {}),
        ...(target.seasonNumber !== undefined ? { season: target.seasonNumber } : {}),
        ...(target.seed
          ? {
              title: target.seed.title,
              original_title: target.seed.original_title,
              overview: target.seed.overview,
              release_date: target.seed.release_date,
              poster_path: target.seed.poster_path,
              original_language: target.seed.original_language,
              type: target.seed.type,
            }
          : {}),
      },
    });
  const openPerson = (tmdbID: number) =>
    void navigate({ to: `/people/${tmdbID}`, search: { from: currentHref } });
  const [view, setView] = useState("now");
  const [online, setOnline] = useState(() => navigator.onLine);
  const [checkingConnection, setCheckingConnection] = useState(false);
  const logout = useMutation({
    mutationFn: endCurrentSession,
    onSuccess: () => clearSignedInCache(queryClient),
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
    <Button
      className="bottom-nav-button"
      variant={tab === value ? "light" : "subtle"}
      leftSection={<Icon size={18} stroke={1.8} />}
      onClick={() =>
        void navigate({
          to: value === "search" ? "/discover" : value === "library" ? "/profile" : `/${value}`,
        })
      }
    >
      {label}
    </Button>
  );

  return (
    <AppShell className="visto-shell" footer={{ height: 76 }} padding={0}>
      <AppShell.Main className="visto-main">
        {!online && (
          <Alert color="yellow" mb="md">
            <Group justify="space-between" align="center">
              <Text size="sm">
                You are offline. Saved information may be out of date, and changes need a
                connection.
              </Text>
              <Button
                size="xs"
                variant="default"
                loading={checkingConnection}
                onClick={() => void retryConnection()}
              >
                Retry connection
              </Button>
            </Group>
          </Alert>
        )}
        {personID ? (
          <Deferred>
            <PersonDetailPage
              personID={personID}
              onBack={() => void navigate({ to: safeReturnPath(returnTo, "/discover") })}
              onOpenDetail={openDetail}
            />
          </Deferred>
        ) : detail ? (
          <Deferred>
            <MediaDetailPage
              target={detail}
              onBack={() =>
                void navigate({
                  to: safeReturnPath(
                    returnTo,
                    detail.mediaType === "tv" ? "/profile" : "/discover",
                  ),
                })
              }
              onOpenDetail={openDetail}
              onOpenPerson={openPerson}
            />
          </Deferred>
        ) : listStatus ? (
          <Deferred>
            <LibraryListPage
              status={listStatus}
              onBack={() => void navigate({ to: "/profile" })}
              onOpenDetail={openDetail}
            />
          </Deferred>
        ) : (
          tab === "watch" && (
            <>
              <Tabs
                className="section-tabs"
                value={view}
                onChange={(value) => setView(value || "now")}
              >
                <Tabs.List>
                  <Tabs.Tab value="now">To watch</Tabs.Tab>
                  <Tabs.Tab value="calendar" leftSection={<IconCalendar size={16} />}>
                    Calendar
                  </Tabs.Tab>
                </Tabs.List>
              </Tabs>
              {view === "now" ? <WatchNow onOpenDetail={openDetail} /> : <WatchCalendar />}
            </>
          )
        )}
        {!detail && !personID && tab === "search" && (
          <Deferred>
            <SearchPanel onOpenDetail={openDetail} />
          </Deferred>
        )}
        {!detail && !personID && tab === "feed" && (
          <Deferred>
            <FeedArea />
          </Deferred>
        )}
        {!detail && !personID && !listStatus && tab === "library" && (
          <Deferred>
            <LibraryArea
              user={user}
              theme={theme}
              onThemeChange={setTheme}
              onSignOut={() => logout.mutate()}
              signingOut={logout.isPending}
              signOutError={logout.isError ? logout.error.message : undefined}
              onOpenDetail={openDetail}
              onOpenList={(status) => void navigate({ to: `/profile/library/${status}` })}
            />
          </Deferred>
        )}
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
  return (
    <Suspense
      fallback={
        <Center py="xl">
          <Loader size="sm" />
        </Center>
      }
    >
      {children}
    </Suspense>
  );
}

function safeReturnPath(returnTo: string | null | undefined, fallback: string): string {
  if (!returnTo) return fallback;
  try {
    const url = new URL(returnTo, window.location.origin);
    return url.origin === window.location.origin
      ? `${url.pathname}${url.search}${url.hash}`
      : fallback;
  } catch {
    return fallback;
  }
}
