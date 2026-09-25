import {
  useGetDiscoverMediaTypeTmdbIDRelated,
  useGetDiscoverMoviesTmdbID,
  useGetDiscoverShowsTmdbID,
  useGetDiscoverShowsTmdbIDSeasonsSeasonNumber,
  useGetDiscoverShowsTmdbIDSeasonsSeasonNumberEpisodesEpisodeNumber,
  useGetEpisodesEpisodeIDRating,
  useGetMoviesTmdbID,
  useGetPlays,
  useGetShowsShowIDEpisodes,
  useGetShowsTmdbID,
} from "../../generated/api";
import { retryTransientRequest } from "../../lib/api";
import { useUserQueryKey } from "../auth/SessionContext";
import type { MediaDetailTarget } from "../../types";

const retryOptions = { retry: retryTransientRequest, retryDelay: (attempt: number) => Math.min(500 * 2 ** attempt, 3000) };

export function useMediaDetailQueries(target: MediaDetailTarget) {
  const userQueryKey = useUserQueryKey();
  const showLibrary = useGetShowsTmdbID(target.tmdbID, { query: {
    queryKey: userQueryKey("media-detail", "tv", target.tmdbID),
    enabled: target.mediaType === "tv",
    ...retryOptions,
  } });
  const movieLibrary = useGetMoviesTmdbID(target.tmdbID, { query: {
    queryKey: userQueryKey("media-detail", "movie", target.tmdbID),
    enabled: target.mediaType === "movie",
    ...retryOptions,
  } });
  const show = useGetDiscoverShowsTmdbID(target.tmdbID, { query: {
    queryKey: userQueryKey("temporary-show-detail", target.tmdbID),
    enabled: target.mediaType === "tv",
    staleTime: 12 * 60 * 60 * 1000,
    ...retryOptions,
  } });
  const movie = useGetDiscoverMoviesTmdbID(target.tmdbID, { query: {
    queryKey: userQueryKey("temporary-movie-detail", target.tmdbID),
    enabled: target.mediaType === "movie",
    staleTime: 12 * 60 * 60 * 1000,
    ...retryOptions,
  } });
  const related = useGetDiscoverMediaTypeTmdbIDRelated(target.mediaType, target.tmdbID, { query: {
    queryKey: userQueryKey("related-media", target.mediaType, target.tmdbID),
    enabled: !target.episodeID,
    staleTime: 12 * 60 * 60 * 1000,
    ...retryOptions,
  } });
  return { library: target.mediaType === "tv" ? showLibrary : movieLibrary, show, movie, related };
}

export function useShowEpisodesQuery(showID: string | undefined, enabled: boolean) {
  const userQueryKey = useUserQueryKey();
  return useGetShowsShowIDEpisodes(showID ?? "", { query: {
    queryKey: userQueryKey("detail-episodes", showID),
    enabled: enabled && Boolean(showID),
    ...retryOptions,
  } });
}

export function useDetailHistoryQuery(enabled: boolean) {
  const userQueryKey = useUserQueryKey();
  return useGetPlays({ limit: 500 }, { query: {
    queryKey: userQueryKey("detail-history"),
    enabled,
    ...retryOptions,
  } });
}

export function useTemporarySeasonEpisodesQuery(target: MediaDetailTarget, enabled: boolean, seasonNumber: number) {
  const userQueryKey = useUserQueryKey();
  return useGetDiscoverShowsTmdbIDSeasonsSeasonNumber(target.tmdbID, seasonNumber, { query: {
    queryKey: userQueryKey("temporary-season-episodes", target.tmdbID, seasonNumber),
    enabled: enabled && target.mediaType === "tv" && Number.isInteger(seasonNumber) && seasonNumber >= 0,
    ...retryOptions,
  } });
}

export function useEpisodeDetailsQuery(target: MediaDetailTarget, seasonNumber: number | undefined, episodeNumber: number | undefined) {
  const userQueryKey = useUserQueryKey();
  return useGetDiscoverShowsTmdbIDSeasonsSeasonNumberEpisodesEpisodeNumber(target.tmdbID, seasonNumber ?? -1, episodeNumber ?? -1, { query: {
    queryKey: userQueryKey("temporary-episode-detail", target.tmdbID, seasonNumber, episodeNumber),
    enabled: target.mediaType === "tv" && Boolean(target.episodeID) && seasonNumber !== undefined && episodeNumber !== undefined,
    ...retryOptions,
  } });
}

export function useEpisodeRatingQuery(episodeID: string | undefined) {
  const userQueryKey = useUserQueryKey();
  return useGetEpisodesEpisodeIDRating(episodeID ?? "", { query: {
    queryKey: userQueryKey("episode-rating", episodeID),
    enabled: Boolean(episodeID),
    ...retryOptions,
  } });
}
