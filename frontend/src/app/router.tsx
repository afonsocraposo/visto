import { createRootRouteWithContext, createRoute, createRouter } from "@tanstack/react-router";
import { z } from "zod";
import { Dashboard, type DashboardPage } from "../features/navigation/Dashboard";
import { LogoutPage } from "../features/auth/LogoutPage";
import type { Theme, User } from "../types";

export type RouterContext = { user: User; theme: Theme; setTheme: (theme: Theme) => void };

const rootRoute = createRootRouteWithContext<RouterContext>()();
const dashboardRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: () => <DashboardRoute page={{ kind: "watch" }} />,
});
const watchRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/watch",
  component: () => <DashboardRoute page={{ kind: "watch" }} />,
});
const discoverRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/discover",
  component: () => <DashboardRoute page={{ kind: "discover" }} />,
});
const feedRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/feed",
  component: () => <DashboardRoute page={{ kind: "feed" }} />,
});
const profileRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/profile",
  component: () => <DashboardRoute page={{ kind: "profile" }} />,
});
const logoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/logout",
  component: LogoutPage,
});

const statusSchema = z.enum(["watching", "completed", "watchlist", "paused", "dropped"]);
const profileLibraryListRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/profile/library/$status",
  component: LibraryListRoute,
  params: {
    parse: (params) => ({ status: statusSchema.parse(params.status) }),
    stringify: (params) => ({ status: params.status }),
  },
});
const mediaSearchSchema = z.object({
  from: z.string().max(2048).optional(),
  media: z.string().optional(),
  episode: z.string().optional(),
  season: z.coerce.number().int().nonnegative().optional(),
  title: z.string().optional(),
  original_title: z.string().optional(),
  overview: z.string().optional(),
  release_date: z.string().optional(),
  poster_path: z.string().optional(),
  original_language: z.string().optional(),
  type: z.enum(["movie", "tv"]).optional(),
});
const mediaRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/media/$mediaType/$tmdbID",
  component: MediaRoute,
  params: {
    parse: (params) => ({
      mediaType: z.enum(["movie", "tv"]).parse(params.mediaType),
      tmdbID: z.coerce.number().int().positive().parse(params.tmdbID),
    }),
    stringify: (params) => ({ mediaType: params.mediaType, tmdbID: String(params.tmdbID) }),
  },
  validateSearch: (search) => mediaSearchSchema.parse(search),
});
const personRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/people/$tmdbID",
  component: PersonRoute,
  params: {
    parse: (params) => ({ tmdbID: z.coerce.number().int().positive().parse(params.tmdbID) }),
    stringify: (params) => ({ tmdbID: String(params.tmdbID) }),
  },
  validateSearch: (search) => z.object({ from: z.string().max(2048).optional() }).parse(search),
});
const routeTree = rootRoute.addChildren([
  dashboardRoute,
  watchRoute,
  discoverRoute,
  feedRoute,
  profileRoute,
  logoutRoute,
  profileLibraryListRoute,
  mediaRoute,
  personRoute,
]);
export const router = createRouter({ routeTree, context: undefined! });

function DashboardRoute({ page }: { page: DashboardPage }) {
  return <Dashboard {...rootRoute.useRouteContext()} page={page} />;
}
function LibraryListRoute() {
  return (
    <DashboardRoute
      page={{ kind: "library-list", status: profileLibraryListRoute.useParams().status }}
    />
  );
}
function PersonRoute() {
  const params = personRoute.useParams();
  const search = personRoute.useSearch();
  return (
    <DashboardRoute page={{ kind: "person", personID: params.tmdbID, returnTo: search.from }} />
  );
}
function MediaRoute() {
  const params = mediaRoute.useParams();
  const search = mediaRoute.useSearch();
  const seed = search.title
    ? {
        tmdb_id: params.tmdbID,
        type: search.type || params.mediaType,
        title: search.title,
        original_title: search.original_title || search.title,
        overview: search.overview || "",
        release_date: search.release_date || "",
        poster_path: search.poster_path || "",
        original_language: search.original_language || "",
      }
    : undefined;
  return (
    <DashboardRoute
      page={{
        kind: "media",
        target: {
          mediaType: params.mediaType,
          tmdbID: params.tmdbID,
          mediaID: search.media,
          episodeID: search.episode,
          seasonNumber: search.season,
          seed,
        },
        returnTo: search.from,
      }}
    />
  );
}

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
