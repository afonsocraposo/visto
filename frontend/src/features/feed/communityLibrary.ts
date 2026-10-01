import type { LibraryStatus, Tab } from "../../types";
import type { LibrarySort } from "../library/librarySort";
import type { LibraryMediaFilter } from "../library/mediaFilter";

export function communityLibraryURL(
  userID: string,
  options: {
    status: LibraryStatus;
    sort: LibrarySort;
    mediaFilter: LibraryMediaFilter;
    limit?: number;
    query?: string;
  },
): string {
  const params = new URLSearchParams({ status: options.status, sort: options.sort });
  if (options.limit) params.set("limit", String(options.limit));
  if (options.mediaFilter !== "all") params.set("media_type", options.mediaFilter);
  if (options.query) params.set("q", options.query);
  return `/api/v1/community/users/${encodeURIComponent(userID)}/library?${params}`;
}

export function communityLibraryListPath(
  userID: string,
  status: LibraryStatus,
  search: { mediaFilter?: LibraryMediaFilter; query?: string; tab?: Tab },
): string {
  const params = new URLSearchParams();
  if (search.mediaFilter && search.mediaFilter !== "all")
    params.set("media_type", search.mediaFilter);
  if (search.query) params.set("q", search.query);
  if (search.tab) params.set("tab", search.tab);
  const query = params.toString();
  return `/users/${encodeURIComponent(userID)}/library/${status}${query ? `?${query}` : ""}`;
}

/** "João's library", "James's library". */
export function possessive(name: string): string {
  return `${name.trim()}'s`;
}
