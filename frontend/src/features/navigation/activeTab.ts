import type { Tab } from "../../types";

const TABS: readonly Tab[] = ["watch", "search", "feed", "library"];

export function activeTabForLocation(pathname: string, search: string): Tab {
  if (pathname.startsWith("/media/") || pathname.startsWith("/people/")) {
    const raw = new URLSearchParams(search).get("tab");
    return isTab(raw) ? raw : "watch";
  }
  return tabForPath(pathname);
}

function isTab(value: string | null): value is Tab {
  return value !== null && (TABS as readonly string[]).includes(value);
}

function tabForPath(pathname: string): Tab {
  if (pathname.startsWith("/discover")) return "search";
  if (pathname.startsWith("/feed")) return "feed";
  if (pathname.startsWith("/profile")) return "library";
  return "watch";
}
