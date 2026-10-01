import { useMutation, useQuery } from "@tanstack/react-query";
import { postLibrary, postPlays, useGetSearch, useGetTrending } from "../../generated/api";
import { retryTransientRequest } from "../../lib/api";
import { api } from "../../lib/api";
import { showActionFeedback } from "../../lib/actionFeedback";
import { useInvalidateUserCache, userCache } from "../../lib/userCache";
import { useUserQueryKey } from "../auth/SessionContext";
import type { SearchMedia, LibraryEntry } from "../../types";
import type { Play } from "../../generated/models/play";
import { filterByMediaType, type DiscoverMediaType } from "./discoverMediaType";

export function useDiscoverQueries(query: string, mediaType: DiscoverMediaType) {
  const userQueryKey = useUserQueryKey();
  const results = useGetSearch(
    { q: query, ...(mediaType === "all" ? {} : { type: mediaType }) },
    {
      query: {
        queryKey: userQueryKey("search", query, mediaType),
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
  const visible = query
    ? filterByMediaType(results.data ?? [], mediaType)
    : [
        ...(mediaType === "movie" ? [] : (trending.data?.tv ?? []).slice(0, 10)),
        ...(mediaType === "tv" ? [] : (trending.data?.movies ?? []).slice(0, 10)),
      ];
  const mediaIDs = visible.map((item) => `${item.type}:${item.tmdb_id}`);
  const library = useQuery({
    queryKey: [...userQueryKey("library"), "lookup", mediaIDs.join(",")],
    enabled: mediaIDs.length > 0,
    queryFn: async () => {
      const batches: LibraryEntry[][] = [];
      for (let start = 0; start < mediaIDs.length; start += 100) {
        batches.push(
          await api.post<LibraryEntry[]>("/api/v1/library/lookup", {
            media_ids: mediaIDs.slice(start, start + 100),
          }),
        );
      }
      return batches.flat();
    },
  });
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
      await postLibrary({ media, status: "watchlist" });
      try {
        return await postPlays({ media_id: `${media.type}:${media.tmdb_id}` });
      } catch (error) {
        await api
          .delete(`/api/v1/library/${encodeURIComponent(`movie:${media.tmdb_id}`)}`)
          .catch(() => undefined);
        throw error;
      }
    },
    onSuccess: (play, media) => {
      void invalidate(userCache.library, userCache.history, userCache.feed);
      showActionFeedback(`${media.title} marked watched.`, async () => {
        await api.delete(`/api/v1/plays/${encodeURIComponent(play.id)}`);
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
        await postLibrary({ media, status: "watchlist", ...(rating === null ? {} : { rating }) });
        await invalidate(userCache.library, userCache.history, userCache.feed);
      });
    },
  });
  return { addToLibrary, addMovieAsWatched, markWatchlistMovieWatched };
}
