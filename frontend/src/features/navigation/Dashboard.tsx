import { lazy, Suspense, useEffect, useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Alert, AppShell, Button, Group, Modal, Text } from "@mantine/core";
import { IconCalendar } from "@tabler/icons-react";
import { WatchCalendar, WatchNow } from "../watch/Watch";
import { SectionTabs } from "../../components/SectionTabs";
import { PageLoadingShell } from "../../components/LoadingShell";
import { AccountAvatar } from "../../components/AccountAvatar";
import { RouteLink } from "../../components/RouteLink";
import { navItems } from "./navItems";
import { requestUpNextScroll } from "../watch/upNextScroll";
import { WatchHistoryReveal } from "../watch/WatchHistoryReveal";
import type { LibraryStatus, MediaDetailTarget, Tab, Theme, User } from "../../types";
import type { LibraryMediaFilter } from "../library/mediaFilter";
import { api, connectionUnavailableEvent } from "../../lib/api";
import { clearSignedInCache, endCurrentSession } from "../auth/logout";
import { readStoredChoice, writeStoredChoice } from "../../lib/browserStorage";
import { ImportData } from "../library/ImportData";
import { useUserQueryKey } from "../auth/SessionContext";
import { useAppNavigation } from "./useAppNavigation";

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
const SettingsPage = lazy(async () => ({
  default: (await import("../library/SettingsPage")).SettingsPage,
}));

export type DashboardPage =
  | { kind: "watch" | "discover" | "feed" | "profile" | "settings" }
  | { kind: "library-list"; status: LibraryStatus; mediaFilter: LibraryMediaFilter; query: string }
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
  const detail = page.kind === "media" ? page.target : null;
  const personID = page.kind === "person" ? page.personID : undefined;
  const profileUserID = page.kind === "user-profile" ? page.userID : undefined;
  const listStatus = page.kind === "library-list" ? page.status : undefined;
  const returnTo =
    page.kind === "media" || page.kind === "person" || page.kind === "user-profile"
      ? page.returnTo
      : undefined;
  const { tab, openDetail, openPerson, openUser, goBack } = useAppNavigation(user, returnTo);
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

  const settingsOpen = page.kind === "settings";
  // One routing implementation for both bars; only the presentation differs.
  const goToTab = (value: Tab) => {
    if (value === "watch") requestUpNextScroll();
    void navigate({ to: navItems.find((item) => item.tab === value)!.path });
  };
  // The top bar marks Settings on the avatar; the bottom bar keeps Library active for it.
  const navButtons = (className: string, iconSize: number, settingsHasOwnMarker: boolean) =>
    navItems.map(({ tab: value, label, path, Icon }) => (
      <RouteLink
        key={value}
        href={path}
        className={className}
        aria-current={tab === value && !(settingsOpen && settingsHasOwnMarker) ? "page" : undefined}
        onOpen={() => goToTab(value)}
      >
        <span className="nav-icon" aria-hidden="true">
          <Icon size={iconSize} stroke={1.8} />
        </span>
        <span className="nav-label">{label}</span>
      </RouteLink>
    ));
  const pageContent = !profileUserID && !detail && !personID && !listStatus && !settingsOpen;

  return (
    <AppShell
      className="visto-shell"
      header={{ height: "var(--visto-header-height)" }}
      footer={{ height: "var(--visto-footer-height)" }}
      padding={0}
    >
      <AppShell.Header className="visto-topbar">
        <div className="topbar-inner">
          <RouteLink href="/watch" className="topbar-brand" onOpen={() => goToTab("watch")}>
            <img src="/icon.svg?v=3" alt="" aria-hidden="true" />
            <span>Visto</span>
          </RouteLink>
          <nav className="topbar-nav" aria-label="Main navigation">
            {navButtons("topbar-nav-button", 18, true)}
          </nav>
          <RouteLink
            href="/settings"
            className="topbar-account"
            aria-label="Account and settings"
            aria-current={settingsOpen ? "page" : undefined}
            onOpen={() => void navigate({ to: "/settings" })}
          >
            <AccountAvatar name={user.name} size={34} />
          </RouteLink>
        </div>
      </AppShell.Header>
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
        {settingsOpen ? (
          <Deferred>
            <SettingsPage
              user={user}
              theme={theme}
              onThemeChange={setTheme}
              onSignOut={() => logout.mutate()}
              signingOut={logout.isPending}
              signOutError={logout.isError ? logout.error.message : undefined}
              onBack={() => goBack("/profile")}
            />
          </Deferred>
        ) : profileUserID ? (
          <Deferred>
            <UserProfilePage
              userID={profileUserID}
              onBack={() => goBack("/feed")}
              onOpenDetail={openDetail}
            />
          </Deferred>
        ) : personID ? (
          <Deferred>
            <PersonDetailPage
              personID={personID}
              onBack={() => goBack("/discover")}
              onOpenDetail={openDetail}
            />
          </Deferred>
        ) : detail ? (
          <Deferred>
            <MediaDetailPage
              target={detail}
              returnTo={returnTo}
              onBack={() => goBack(detail.mediaType === "tv" ? "/profile" : "/discover")}
              onOpenDetail={openDetail}
              onOpenPerson={openPerson}
            />
          </Deferred>
        ) : listStatus ? (
          <Deferred>
            <LibraryListPage
              status={listStatus}
              mediaFilter={page.kind === "library-list" ? page.mediaFilter : "all"}
              query={page.kind === "library-list" ? page.query : ""}
              onQueryChange={(query) =>
                void navigate({
                  to: "/profile/library/$status",
                  params: { status: listStatus },
                  search: {
                    media_type:
                      page.kind === "library-list" && page.mediaFilter !== "all"
                        ? page.mediaFilter
                        : undefined,
                    q: query || undefined,
                  },
                  replace: true,
                  resetScroll: false,
                })
              }
              onMediaFilterChange={(mediaFilter) =>
                void navigate({
                  to: "/profile/library/$status",
                  params: { status: listStatus },
                  search: {
                    media_type: mediaFilter === "all" ? undefined : mediaFilter,
                    q: page.kind === "library-list" ? page.query || undefined : undefined,
                  },
                  replace: true,
                })
              }
              onBack={() => goBack("/profile")}
              onOpenDetail={openDetail}
            />
          </Deferred>
        ) : (
          tab === "watch" &&
          !settingsOpen && (
            <>
              {view === "now" && <WatchHistoryReveal key="history" onOpenDetail={openDetail} />}
              <SectionTabs
                key="tabs"
                label="Watching views"
                value={view}
                onChange={(next) => {
                  setView(next);
                  writeStoredChoice("session", watchTabStorageKey, next);
                }}
                options={[
                  { value: "now", label: "To watch" },
                  { value: "calendar", label: "Upcoming", icon: <IconCalendar size={16} /> },
                ]}
              />
              <div key={view} className="section-panel watch-up-next">
                {view === "now" ? (
                  <WatchNow onOpenDetail={openDetail} />
                ) : (
                  <WatchCalendar onOpenDetail={openDetail} />
                )}
              </div>
            </>
          )
        )}
        {pageContent && tab === "search" && (
          <Deferred>
            <SearchPanel onOpenDetail={openDetail} />
          </Deferred>
        )}
        {pageContent && tab === "feed" && (
          <Deferred>
            <FeedArea userID={user.id} onOpenDetail={openDetail} onOpenUser={openUser} />
          </Deferred>
        )}
        {pageContent && tab === "library" && (
          <Deferred>
            <LibraryArea
              user={user}
              onOpenSettings={() => void navigate({ to: "/settings" })}
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
        <nav className="bottom-nav" aria-label="Main navigation">
          {navButtons("bottom-nav-button", 22, false)}
        </nav>
      </AppShell.Footer>
    </AppShell>
  );
}

function Deferred({ children }: { children: ReactNode }) {
  return <Suspense fallback={<PageLoadingShell />}>{children}</Suspense>;
}
