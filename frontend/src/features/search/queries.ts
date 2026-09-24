import { useMutation, useQuery } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { useInvalidateUserCache, userCache } from "../../lib/userCache";
import { useUserQueryKey } from "../auth/SessionContext";
import type { LibraryEntry, SearchMedia, TrendingResponse } from "../../types";

export function useDiscoverQueries(query: string) {
  const userQueryKey = useUserQueryKey();
  const library = useQuery({ queryKey: userQueryKey("library"), queryFn: () => api.get<LibraryEntry[]>("/api/v1/library", "Could not load your library.") });
  const results = useQuery({
    queryKey: userQueryKey("search", query),
    enabled: query.length > 1,
    queryFn: () => api.get<SearchMedia[]>(`/api/v1/search?q=${encodeURIComponent(query)}`, "Search is temporarily unavailable."),
  });
  const trending = useQuery({
    queryKey: userQueryKey("trending", "week"),
    enabled: !query,
    queryFn: () => api.get<TrendingResponse>("/api/v1/trending?window=week", "Trending media is temporarily unavailable."),
    staleTime: 5 * 60 * 1000,
  });
  return { library, results, trending };
}

export function useDiscoverMutations() {
  const invalidate = useInvalidateUserCache();
  const addToLibrary = useMutation({
    mutationFn: ({ media, status }: { media: SearchMedia; status: "watching" | "watchlist" }) => api.post("/api/v1/library", { media, status }, "Could not add this title."),
    onSuccess: () => invalidate(userCache.library, userCache.continue, userCache.calendar),
  });
  const addMovieAsWatched = useMutation({
    mutationFn: async (media: SearchMedia) => {
      await api.post("/api/v1/library", { media, status: "watching" }, "Could not add this title.");
      await api.post("/api/v1/plays", { media_id: `${media.type}:${media.tmdb_id}` }, "Could not record this watch.");
    },
    onSuccess: () => invalidate(userCache.library, userCache.history, userCache.feed),
  });
  return { addToLibrary, addMovieAsWatched };
}
