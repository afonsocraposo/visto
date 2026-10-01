export type DiscoverMediaType = "all" | "tv" | "movie";

export const discoverMediaTypeOptions = [
  { value: "all", label: "All" },
  { value: "tv", label: "Series" },
  { value: "movie", label: "Movies" },
];

// Keeps an older or concurrent response from leaking the other type into a filtered view.
export function filterByMediaType<T extends { type: string }>(
  items: T[],
  mediaType: DiscoverMediaType,
): T[] {
  return mediaType === "all" ? items : items.filter((item) => item.type === mediaType);
}

export function emptyResultsTitle(mediaType: DiscoverMediaType, query: string) {
  if (mediaType === "all") return `No results for “${query}”`;
  return `No ${mediaType === "tv" ? "series" : "movies"} found for “${query}”`;
}
