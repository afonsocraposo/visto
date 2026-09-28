import { lazy, Suspense, useEffect, useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useRouterState } from "@tanstack/react-router";
import { Alert, AppShell, Button, Center, Group, Loader, Modal, Tabs, Text } from "@mantine/core";
import {
  IconCalendar,
  IconCompass,
  IconHome,
  IconSearch,
  IconUserCircle,
} from "@tabler/icons-react";
import { WatchNow, WatchCalendar } from "../watch/Watch";
import type { LibraryStatus, MediaDetailTarget, Tab, Theme, User } from "../../types";
import type { LibraryMediaFilter } from "../library/mediaFilter";
import { api, connectionUnavailableEvent } from "../../lib/api";
import { clearSignedInCache, endCurrentSession } from "../auth/logout";
import { readStoredChoice, writeStoredChoice } from "../../lib/browserStorage";
import { ImportData } from "../library/ImportData";
import { useUserQueryKey } from "../auth/SessionContext";

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
const UserProfilePage = lazy(async () => ({
  default: (await import("../feed/UserProfilePage")).UserProfilePage,
}));

export type DashboardPage =
  | { kind: "watch" | "discover" | "feed" | "profile" }
  | { kind: "library-list"; status: LibraryStatus; mediaFilter: LibraryMediaFilter }
  | { kind: "media"; target: MediaDetailTarget; returnTo?: string }
  | { kind: "person"; personID: number; returnTo?: string }
  | { kind: "user-profile"; userID: string; returnTo?: string };

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
  const userQueryKey = useUserQueryKey();
  const importWelcome = useQuery({
    queryKey: userQueryKey("import-welcome"),
    queryFn: () => api.get<{ pending: boolean }>("/api/v1/imports/welcome"),
  });
  const dismissImport = useMutation({
    mutationFn: () => api.post("/api/v1/imports/welcome/dismiss"),
    onSuccess: () => queryClient.setQueryData(userQueryKey("import-welcome"), { pending: false }),
  });
  const navigate = useNavigate();
  const currentHref = useRouterState({ select: (state) => state.location.href });
  const detail = page.kind === "media" ? page.target : null;
  const personID = page.kind === "person" ? page.personID : undefined;
  const profileUserID = page.kind === "user-profile" ? page.userID : undefined;
  const listStatus = page.kind === "library-list" ? page.status : undefined;
  const returnTo =
    page.kind === "media" || page.kind === "person" || page.kind === "user-profile"
      ? page.returnTo
      : undefined;
  const tab: Tab =
    page.kind === "discover"
      ? "search"
      : page.kind === "feed"
        ? "feed"
        : page.kind === "user-profile"
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
      },
    });
  const openPerson = (tmdbID: number) =>
    void navigate({ to: `/people/${tmdbID}`, search: { from: currentHref } });
  const openUser = (userID: string) =>
    void navigate(
      userID === user.id
        ? { to: "/profile" }
        : { to: "/users/$userID", params: { userID }, search: { from: currentHref } },
    );
  const watchTabStorageKey = `visto:tab:${user.id}:watching`;
  const [view, setView] = useState(() =>
    readStoredChoice("session", watchTabStorageKey, ["now", "calendar"] as const, "now"),
  );
  const [online, setOnline] = useState(() => navigator.onLine);
  const [checkingConnection, setCheckingConnection] = useState(false);
  const logout = useMutation({
    mutationFn: endCurrentSession,
    onSuccess: () => clearSignedInCache(queryClient),
  });

  const verifyConnection = async (refreshQueries = false) => {
    try {
      const response = await fetch("/health", { cache: "no-store" });
      if (!response.ok) throw new Error("Server is not ready.");
      setOnline(true);
      if (refreshQueries) await queryClient.invalidateQueries({ queryKey: ["user", user.id] });
    } catch {
      setOnline(false);
    }
  };

  const retryConnection = async () => {
    setCheckingConnection(true);
    try {
      await verifyConnection(true);
    } finally {
      setCheckingConnection(false);
    }
  };

  useEffect(() => {
    const onlineHandler = () => void retryConnection();
    const offlineHandler = () => setOnline(false);
    const unavailableHandler = () => void verifyConnection();
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
      <Modal
        opened={importWelcome.data?.pending === true}
        onClose={() => dismissImport.mutate()}
        title="Bring your data to Visto"
        centered
      >
        <ImportData withinModal />
        <Button
          variant="subtle"
          mt="md"
          onClick={() => dismissImport.mutate()}
          loading={dismissImport.isPending}
        >
          Continue to Visto
        </Button>
        {dismissImport.isError && (
          <Alert color="red" mt="sm">
            Could not close this prompt. Try again.
          </Alert>
        )}
      </Modal>
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
        {profileUserID ? (
          <Deferred>
            <UserProfilePage
              userID={profileUserID}
              onBack={() => void navigate({ to: safeReturnPath(returnTo, "/feed") })}
              onOpenDetail={openDetail}
            />
          </Deferred>
        ) : personID ? (
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
              mediaFilter={page.kind === "library-list" ? page.mediaFilter : "all"}
              onMediaFilterChange={(mediaFilter) =>
                void navigate({
                  to: "/profile/library/$status",
                  params: { status: listStatus },
                  search: { media_type: mediaFilter === "all" ? undefined : mediaFilter },
                  replace: true,
                })
              }
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
                onChange={(value) => {
                  const next = value === "calendar" ? "calendar" : "now";
                  setView(next);
                  writeStoredChoice("session", watchTabStorageKey, next);
                }}
              >
                <Tabs.List>
                  <Tabs.Tab value="now">To watch</Tabs.Tab>
                  <Tabs.Tab value="calendar" leftSection={<IconCalendar size={16} />}>
                    Upcoming
                  </Tabs.Tab>
                </Tabs.List>
              </Tabs>
              {view === "now" ? (
                <WatchNow onOpenDetail={openDetail} />
              ) : (
                <WatchCalendar onOpenDetail={openDetail} />
              )}
            </>
          )
        )}
        {!profileUserID && !detail && !personID && tab === "search" && (
          <Deferred>
            <SearchPanel onOpenDetail={openDetail} />
          </Deferred>
        )}
        {!profileUserID && !detail && !personID && tab === "feed" && (
          <Deferred>
            <FeedArea userID={user.id} onOpenDetail={openDetail} onOpenUser={openUser} />
          </Deferred>
        )}
        {!profileUserID && !detail && !personID && !listStatus && tab === "library" && (
          <Deferred>
            <LibraryArea
              user={user}
              theme={theme}
              onThemeChange={setTheme}
              onSignOut={() => logout.mutate()}
              signingOut={logout.isPending}
              signOutError={logout.isError ? logout.error.message : undefined}
              onOpenDetail={openDetail}
              onOpenList={(status, mediaFilter) =>
                void navigate({
                  to: "/profile/library/$status",
                  params: { status },
                  search: { media_type: mediaFilter === "all" ? undefined : mediaFilter },
                })
              }
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
