import { useMutation } from "@tanstack/react-query";
import {
  postLibrary,
  postPlays,
  useGetLibrary,
  useGetSearch,
  useGetTrending,
} from "../../generated/api";
import { retryTransientRequest } from "../../lib/api";
import { api } from "../../lib/api";
import { showActionFeedback } from "../../components/ActionFeedback";
import { useInvalidateUserCache, userCache } from "../../lib/userCache";
import { useUserQueryKey } from "../auth/SessionContext";
import type { SearchMedia } from "../../types";
import type { Play } from "../../generated/models/play";

export function useDiscoverQueries(query: string) {
  const userQueryKey = useUserQueryKey();
  const library = useGetLibrary(undefined, {
    query: {
      queryKey: userQueryKey("library"),
      retry: retryTransientRequest,
    },
  });
  const results = useGetSearch(
    { q: query },
    {
      query: {
        queryKey: userQueryKey("search", query),
        enabled: query.length > 1,
        retry: retryTransientRequest,
        retryDelay: (attempt) => Math.min(500 * 2 ** attempt, 3000),
      },
    },
  );
  const trending = useGetTrending(
    { window: "week" },
    {
      query: {
        queryKey: userQueryKey("trending", "week"),
        enabled: !query,
        staleTime: 5 * 60 * 1000,
        retry: retryTransientRequest,
      },
    },
  );
  return { library, results, trending };
}

export function useDiscoverMutations() {
  const invalidate = useInvalidateUserCache();
  const addToLibrary = useMutation({
    mutationFn: ({ media, status }: { media: SearchMedia; status: "watching" | "watchlist" }) =>
      postLibrary({ media, status }),
    onSuccess: (_result, { media, status }) => {
      void invalidate(userCache.library, userCache.continue, userCache.calendar);
      const mediaID = `${media.type}:${media.tmdb_id}`;
      showActionFeedback(
        status === "watchlist"
          ? `${media.title} added to Watchlist.`
          : `${media.title} added to Watching.`,
        async () => {
          await api.delete(
            `/api/v1/library/${encodeURIComponent(mediaID)}${status === "watching" ? "?status=watching" : ""}`,
          );
          await invalidate(userCache.library, userCache.continue, userCache.calendar);
        },
      );
    },
  });
  const addMovieAsWatched = useMutation({
    mutationFn: async (media: SearchMedia) => {
      await postLibrary({ media, status: "watching" });
      try {
        return await postPlays({ media_id: `${media.type}:${media.tmdb_id}` });
      } catch (error) {
        await api
          .delete(`/api/v1/library/${encodeURIComponent(`movie:${media.tmdb_id}`)}?status=watching`)
          .catch(() => undefined);
        throw error;
      }
    },
    onSuccess: (play, media) => {
      void invalidate(userCache.library, userCache.history, userCache.feed);
      showActionFeedback(`${media.title} marked watched.`, async () => {
        await api.delete(`/api/v1/plays/${encodeURIComponent(play.id)}`);
        await api.delete(
          `/api/v1/library/${encodeURIComponent(`movie:${media.tmdb_id}`)}?status=watching`,
        );
        await invalidate(userCache.library, userCache.history, userCache.feed);
      });
    },
  });
  const markWatchlistMovieWatched = useMutation({
    mutationFn: ({ media }: { media: SearchMedia; rating: number | null }) =>
      api.post<Play>(
        "/api/v1/plays",
        { media_id: `movie:${media.tmdb_id}` },
        "Could not mark this movie watched.",
      ),
    onSuccess: (play, { media, rating }) => {
      void invalidate(userCache.library, userCache.history, userCache.feed);
      showActionFeedback(`${media.title} marked watched.`, async () => {
        await api.delete(`/api/v1/plays/${encodeURIComponent(play.id)}`);
        await api.patch(`/api/v1/library/${encodeURIComponent(`movie:${media.tmdb_id}`)}`, {
          status: "watchlist",
          rating,
        });
        await invalidate(userCache.library, userCache.history, userCache.feed);
      });
    },
  });
  return { addToLibrary, addMovieAsWatched, markWatchlistMovieWatched };
}
