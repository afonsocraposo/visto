import type { Tab } from "../../types";

export function activeTabForLocation(pathname: string, search: string, origin: string): Tab {
  if (pathname.startsWith("/media/") || pathname.startsWith("/people/")) {
    return tabFromReturnLocation(new URLSearchParams(search).get("from"), origin, 0);
  }
  return tabForPath(pathname);
}

function tabFromReturnLocation(returnTo: string | null, origin: string, depth: number): Tab {
  if (!returnTo || depth >= 12) return "watch";
  try {
    const url = new URL(returnTo, origin);
    if (url.origin !== new URL(origin).origin) return "watch";
    if (url.pathname.startsWith("/media/") || url.pathname.startsWith("/people/")) {
      return tabFromReturnLocation(url.searchParams.get("from"), origin, depth + 1);
    }
    return tabForPath(url.pathname);
  } catch {
    return "watch";
  }
}

function tabForPath(pathname: string): Tab {
  if (pathname.startsWith("/discover")) return "search";
  if (pathname.startsWith("/feed")) return "feed";
  if (pathname.startsWith("/profile")) return "library";
  return "watch";
}
