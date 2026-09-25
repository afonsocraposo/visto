import { useMutation } from "@tanstack/react-query";
import { postLibrary, postPlays, useGetLibrary, useGetSearch, useGetTrending } from "../../generated/api";
import { retryTransientRequest } from "../../lib/api";
import { useInvalidateUserCache, userCache } from "../../lib/userCache";
import { useUserQueryKey } from "../auth/SessionContext";
import type { SearchMedia } from "../../types";

export function useDiscoverQueries(query: string) {
  const userQueryKey = useUserQueryKey();
  const library = useGetLibrary({ query: {
    queryKey: userQueryKey("library"),
    retry: retryTransientRequest,
  } });
  const results = useGetSearch({ q: query }, { query: {
    queryKey: userQueryKey("search", query),
    enabled: query.length > 1,
    retry: retryTransientRequest,
    retryDelay: attempt => Math.min(500 * 2 ** attempt, 3000),
  } });
  const trending = useGetTrending({ window: "week" }, { query: {
    queryKey: userQueryKey("trending", "week"),
    enabled: !query,
    staleTime: 5 * 60 * 1000,
    retry: retryTransientRequest,
  } });
  return { library, results, trending };
}

export function useDiscoverMutations() {
  const invalidate = useInvalidateUserCache();
  const addToLibrary = useMutation({
    mutationFn: ({ media, status }: { media: SearchMedia; status: "watching" | "watchlist" }) => postLibrary({ media, status }),
    onSuccess: () => invalidate(userCache.library, userCache.continue, userCache.calendar),
  });
  const addMovieAsWatched = useMutation({
    mutationFn: async (media: SearchMedia) => {
      await postLibrary({ media, status: "watching" });
      await postPlays({ media_id: `${media.type}:${media.tmdb_id}` });
    },
    onSuccess: () => invalidate(userCache.library, userCache.history, userCache.feed),
  });
  return { addToLibrary, addMovieAsWatched };
}
