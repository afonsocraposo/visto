import type { LibraryStatus } from "../../types";

export type LibraryMediaFilter = "all" | "movie" | "tv";

export const libraryMediaFilterOptions = [
  { value: "all", label: "All" },
  { value: "tv", label: "TV" },
  { value: "movie", label: "Movies" },
];

/** Movies can only be in Watchlist or Completed, so the other lists never hold one. */
export function statusAppliesToFilter(
  status: LibraryStatus,
  mediaFilter: LibraryMediaFilter,
): boolean {
  return mediaFilter !== "movie" || status === "watchlist" || status === "completed";
}
