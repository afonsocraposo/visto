import { useNavigate, useRouter, useRouterState } from "@tanstack/react-router";
import type { MediaDetailTarget, Tab, User } from "../../types";
import { activeTabForLocation } from "./activeTab";

export function useAppNavigation(user: User, returnTo: string | undefined) {
  const navigate = useNavigate();
  const router = useRouter();
  const currentHref = useRouterState({ select: (state) => state.location.href });
  const openedInApp = useRouterState({
    select: (state) => state.location.state.vistoOpenedInApp === true,
  });
  const currentURL = new URL(currentHref, window.location.origin);
  const tab = activeTabForLocation(currentURL.pathname, currentURL.search);
  const parentPath = withoutFromParam(currentURL);

  const openDetail = (
    target: MediaDetailTarget,
    options?: { from?: string; tab?: Tab; replace?: boolean; resetScroll?: boolean },
  ) => {
    const episodePath =
      target.mediaType === "tv" &&
      target.seasonNumber !== undefined &&
      target.episodeNumber !== undefined;
    void navigate({
      to: episodePath
        ? `/shows/${target.tmdbID}/season/${target.seasonNumber}/episode/${target.episodeNumber}`
        : `/media/${target.mediaType}/${target.tmdbID}`,
      search: {
        from: options?.from ?? parentPath,
        tab: options?.tab ?? tab,
        ...(target.mediaID ? { media: target.mediaID } : {}),
        ...(target.episodeID ? { episode: target.episodeID } : {}),
        ...(!episodePath && target.seasonNumber !== undefined
          ? { season: target.seasonNumber }
          : {}),
        ...(!episodePath && target.episodeNumber !== undefined
          ? { episode_number: target.episodeNumber }
          : {}),
      },
      replace: options?.replace,
      resetScroll: options?.resetScroll,
      state: { vistoOpenedInApp: options?.replace ? openedInApp : true },
    });
  };
  const openPerson = (tmdbID: number) =>
    void navigate({
      to: `/people/${tmdbID}`,
      search: { from: parentPath, tab },
      state: { vistoOpenedInApp: true },
    });
  const openUser = (userID: string) =>
    void navigate(
      userID === user.id
        ? { to: "/profile" }
        : {
            to: "/users/$userID",
            params: { userID },
            search: { from: parentPath, tab },
            state: { vistoOpenedInApp: true },
          },
    );
  const goBack = (fallback: string) => {
    if (openedInApp && router.history.canGoBack()) router.history.back();
    else void navigate({ to: safeReturnPath(returnTo, window.location.origin) ?? fallback });
  };

  return { tab, openDetail, openPerson, openUser, goBack };
}

function withoutFromParam(url: URL): string {
  const clean = new URL(url);
  clean.searchParams.delete("from");
  return `${clean.pathname}${clean.search}${clean.hash}`;
}

function safeReturnPath(returnTo: string | null | undefined, origin: string): string | null {
  if (!returnTo) return null;
  try {
    const url = new URL(returnTo, origin);
    return url.origin === origin ? `${url.pathname}${url.search}${url.hash}` : null;
  } catch {
    return null;
  }
}
