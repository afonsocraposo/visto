import { createRootRoute, createRootRouteWithContext, createRoute, createRouter } from "@tanstack/react-router";
import { Dashboard } from "../features/navigation/Dashboard";
import type { Theme, User } from "../types";

export type RouterContext = { user: User; theme: Theme; setTheme: (theme: Theme) => void };

const rootRoute = createRootRouteWithContext<RouterContext>()();
const dashboardRoute = createRoute({ getParentRoute: () => rootRoute, path: "/", component: DashboardRoute });
const watchRoute = createRoute({ getParentRoute: () => rootRoute, path: "/watch", component: DashboardRoute });
const discoverRoute = createRoute({ getParentRoute: () => rootRoute, path: "/discover", component: DashboardRoute });
const feedRoute = createRoute({ getParentRoute: () => rootRoute, path: "/feed", component: DashboardRoute });
const profileRoute = createRoute({ getParentRoute: () => rootRoute, path: "/profile", component: DashboardRoute });
const mediaRoute = createRoute({ getParentRoute: () => rootRoute, path: "/media/$mediaType/$tmdbID", component: DashboardRoute });

const routeTree = rootRoute.addChildren([dashboardRoute, watchRoute, discoverRoute, feedRoute, profileRoute, mediaRoute]);
export const router = createRouter({ routeTree, context: undefined! });

function DashboardRoute() {
  const context = rootRoute.useRouteContext();
  return <Dashboard {...context} />;
}

declare module "@tanstack/react-router" {
  interface Register { router: typeof router; }
}
