import {
  useGetDiscoverMediaTypeTmdbIDRelated,
  useGetDiscoverMoviesTmdbID,
  useGetDiscoverShowsTmdbID,
  useGetDiscoverShowsTmdbIDSeasonsSeasonNumber,
  useGetDiscoverShowsTmdbIDSeasonsSeasonNumberEpisodesEpisodeNumber,
  useGetEpisodesEpisodeIDRating,
  useGetMoviesTmdbID,
  useGetShowsTmdbID,
} from "../../generated/api";
import { APIRequestError, api, retryTransientRequest } from "../../lib/api";
import { fetchAllPages, pageURL } from "../../lib/pagination";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import type { CursorPage, HistoryEntry, ShowEpisodeEntry } from "../../types";
import { useUserQueryKey } from "../auth/SessionContext";
import type { MediaDetailTarget } from "../../types";

const retryOptions = {
  retry: retryTransientRequest,
  retryDelay: (attempt: number) => Math.min(500 * 2 ** attempt, 3000),
};

export function useMediaDetailQueries(target: MediaDetailTarget) {
  const userQueryKey = useUserQueryKey();
  const showLibrary = useGetShowsTmdbID(target.tmdbID, {
    query: {
      queryKey: userQueryKey("media-detail", "tv", target.tmdbID),
      enabled: target.mediaType === "tv",
      refetchOnMount: "always",
      ...retryOptions,
    },
  });
  const movieLibrary = useGetMoviesTmdbID(target.tmdbID, {
    query: {
      queryKey: userQueryKey("media-detail", "movie", target.tmdbID),
      enabled: target.mediaType === "movie",
      refetchOnMount: "always",
      ...retryOptions,
    },
  });
  const show = useGetDiscoverShowsTmdbID(target.tmdbID, {
    query: {
      queryKey: userQueryKey("temporary-show-detail", target.tmdbID),
      enabled:
        target.mediaType === "tv" &&
        ((showLibrary.isSuccess && !showLibrary.data?.item.media_id) ||
          (showLibrary.error instanceof APIRequestError && showLibrary.error.status === 404)),
      staleTime: 12 * 60 * 60 * 1000,
      ...retryOptions,
    },
  });
  const movie = useGetDiscoverMoviesTmdbID(target.tmdbID, {
    query: {
      queryKey: userQueryKey("temporary-movie-detail", target.tmdbID),
      enabled:
        target.mediaType === "movie" &&
        ((movieLibrary.isSuccess && !movieLibrary.data?.item.media_id) ||
          (movieLibrary.error instanceof APIRequestError && movieLibrary.error.status === 404)),
      staleTime: 12 * 60 * 60 * 1000,
      ...retryOptions,
    },
  });
  const related = useGetDiscoverMediaTypeTmdbIDRelated(target.mediaType, target.tmdbID, {
    query: {
      queryKey: userQueryKey("related-media", target.mediaType, target.tmdbID),
      enabled: !target.episodeID,
      staleTime: 12 * 60 * 60 * 1000,
      ...retryOptions,
    },
  });
  return { library: target.mediaType === "tv" ? showLibrary : movieLibrary, show, movie, related };
}

export function useShowEpisodesQuery(
  showID: string | undefined,
  seasonID: string | undefined,
  enabled: boolean,
) {
  const userQueryKey = useUserQueryKey();
  return useInfiniteQuery({
    queryKey: userQueryKey("detail-episodes", showID, seasonID),
    enabled: enabled && Boolean(showID && seasonID),
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      api.get<CursorPage<ShowEpisodeEntry>>(
        pageURL(`/api/v1/seasons/${encodeURIComponent(seasonID!)}/episodes`, pageParam),
        "Could not load episodes.",
      ),
    getNextPageParam: (page) => page.next_cursor ?? undefined,
    ...retryOptions,
  });
}

export function useSavedShowEpisodesQuery(showID: string | undefined, enabled: boolean) {
  const userQueryKey = useUserQueryKey();
  return useQuery({
    queryKey: userQueryKey("detail-all-episodes", showID),
    enabled: enabled && Boolean(showID),
    queryFn: () =>
      fetchAllPages<ShowEpisodeEntry>(
        `/api/v1/shows/${encodeURIComponent(showID!)}/episodes`,
        "Could not load episodes.",
      ),
    ...retryOptions,
  });
}

export function useDetailHistoryQuery(enabled: boolean, episodeID?: string, movieID?: string) {
  const userQueryKey = useUserQueryKey();
  const filter = episodeID
    ? `episode_id=${encodeURIComponent(episodeID)}`
    : `media_id=${encodeURIComponent(movieID ?? "")}`;
  return useQuery({
    queryKey: userQueryKey("detail-history", filter),
    enabled: enabled && Boolean(episodeID || movieID),
    queryFn: () =>
      api.get<CursorPage<HistoryEntry>>(
        `/api/v1/plays?${filter}&limit=1`,
        "Could not load watch history.",
      ),
    ...retryOptions,
  });
}

export function useTemporarySeasonEpisodesQuery(
  target: MediaDetailTarget,
  enabled: boolean,
  seasonNumber: number,
) {
  const userQueryKey = useUserQueryKey();
  return useGetDiscoverShowsTmdbIDSeasonsSeasonNumber(target.tmdbID, seasonNumber, {
    query: {
      queryKey: userQueryKey("temporary-season-episodes", target.tmdbID, seasonNumber),
      enabled:
        enabled && target.mediaType === "tv" && Number.isInteger(seasonNumber) && seasonNumber >= 0,
      ...retryOptions,
    },
  });
}

export function useEpisodeDetailsQuery(
  target: MediaDetailTarget,
  seasonNumber: number | undefined,
  episodeNumber: number | undefined,
) {
  const userQueryKey = useUserQueryKey();
  return useGetDiscoverShowsTmdbIDSeasonsSeasonNumberEpisodesEpisodeNumber(
    target.tmdbID,
    seasonNumber ?? -1,
    episodeNumber ?? -1,
    {
      query: {
        queryKey: userQueryKey(
          "temporary-episode-detail",
          target.tmdbID,
          seasonNumber,
          episodeNumber,
        ),
        enabled:
          target.mediaType === "tv" &&
          Boolean(target.episodeID) &&
          seasonNumber !== undefined &&
          episodeNumber !== undefined,
        ...retryOptions,
      },
    },
  );
}

export function useEpisodeRatingQuery(episodeID: string | undefined) {
  const userQueryKey = useUserQueryKey();
  return useGetEpisodesEpisodeIDRating(episodeID ?? "", {
    query: {
      queryKey: userQueryKey("episode-rating", episodeID),
      enabled: Boolean(episodeID),
      ...retryOptions,
    },
  });
}
