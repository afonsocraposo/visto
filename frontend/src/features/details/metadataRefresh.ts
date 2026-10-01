import type { User } from "../../types";

/**
 * Administrators can force-refresh metadata for titles Visto actually stores. Temporary Discover
 * results have nothing to repair, and refreshing them must never create media records.
 */
export function canRefreshMetadata(role: User["role"], savedMediaID: string | undefined): boolean {
  return role === "admin" && Boolean(savedMediaID);
}

/**
 * User-scoped query keys a metadata refresh can change. TV refreshes also rebuild the season and
 * episode catalog, so every episode view of the show is refetched too.
 */
export function metadataRefreshScopes(
  mediaType: "movie" | "tv",
  tmdbID: number,
  showID: string | undefined,
): unknown[][] {
  const scopes: unknown[][] = [["media-detail", mediaType, tmdbID], ["library"], ["continue"]];
  if (mediaType === "tv")
    scopes.push(
      ["show-seasons", showID],
      ["show-progress", showID],
      ["detail-episodes", showID],
      ["detail-all-episodes", showID],
    );
  return scopes;
}

export function metadataRefreshErrorMessage(error: unknown): string {
  const status =
    typeof error === "object" && error !== null && "status" in error ? error.status : undefined;
  if (status === 401 || status === 403) return "Administrator access required.";
  if (status === 404) return "This title is no longer stored in Visto.";
  return "Could not refresh metadata from TMDB.";
}
