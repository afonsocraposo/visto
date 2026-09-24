import { useQuery } from "@tanstack/react-query";
import { api, retryTransientRequest } from "../../lib/api";
import { useUserQueryKey } from "../auth/SessionContext";
import type { EpisodeRating, HistoryEntry, LibraryEntry, MediaDetailTarget, SearchMedia, ShowEpisodeEntry, TemporaryEpisodeDetails, TemporaryMovieDetails, TemporaryShowDetails } from "../../types";

const retryOptions = { retry: retryTransientRequest, retryDelay: (attempt: number) => Math.min(500 * 2 ** attempt, 3000) };

export function useMediaDetailQueries(target: MediaDetailTarget) {
  const userQueryKey = useUserQueryKey();
  const library = useQuery({
    queryKey: userQueryKey("media-detail", target.mediaType, target.tmdbID),
    queryFn: () => api.get<LibraryEntry>(`/api/v1/${target.mediaType === "tv" ? "shows" : "movies"}/${target.tmdbID}`, "Could not load media details."),
    ...retryOptions,
  });
  const show = useQuery({
    queryKey: userQueryKey("temporary-show-detail", target.tmdbID),
    enabled: target.mediaType === "tv",
    queryFn: () => api.get<TemporaryShowDetails>(`/api/v1/discover/shows/${target.tmdbID}`, "Could not load TV show details."),
    ...retryOptions,
  });
  const movie = useQuery({
    queryKey: userQueryKey("temporary-movie-detail", target.tmdbID),
    enabled: target.mediaType === "movie",
    queryFn: () => api.get<TemporaryMovieDetails>(`/api/v1/discover/movies/${target.tmdbID}`, "Could not load movie details."),
    ...retryOptions,
  });
  const related = useQuery({
    queryKey: userQueryKey("related-media", target.mediaType, target.tmdbID),
    enabled: !target.episodeID,
    staleTime: 12 * 60 * 60 * 1000,
    queryFn: () => api.get<SearchMedia[]>(`/api/v1/discover/${target.mediaType}/${target.tmdbID}/related`, "Could not load related titles."),
    ...retryOptions,
  });
  return { library, show, movie, related };
}

export function useShowEpisodesQuery(showID: string | undefined, enabled: boolean) {
  const userQueryKey = useUserQueryKey();
  return useQuery({
    queryKey: userQueryKey("detail-episodes", showID),
    enabled: enabled && Boolean(showID),
    queryFn: () => api.get<ShowEpisodeEntry[]>(`/api/v1/shows/${encodeURIComponent(showID!)}/episodes`, "Could not load episodes."),
  });
}

export function useDetailHistoryQuery(enabled: boolean) {
  const userQueryKey = useUserQueryKey();
  return useQuery({ queryKey: userQueryKey("detail-history"), enabled, queryFn: () => api.get<HistoryEntry[]>("/api/v1/plays?limit=500", "Could not load watch history.") });
}

export function useTemporarySeasonEpisodesQuery(target: MediaDetailTarget, enabled: boolean, seasonNumber: number) {
  const userQueryKey = useUserQueryKey();
  return useQuery({
    queryKey: userQueryKey("temporary-season-episodes", target.tmdbID, seasonNumber),
    enabled: enabled && target.mediaType === "tv" && Number.isInteger(seasonNumber) && seasonNumber >= 0,
    queryFn: () => api.get<{ episodes: ShowEpisodeEntry[] }>(`/api/v1/discover/shows/${target.tmdbID}/seasons/${seasonNumber}`, "Could not load season episodes."),
  });
}

export function useEpisodeDetailsQuery(target: MediaDetailTarget, seasonNumber: number | undefined, episodeNumber: number | undefined) {
  const userQueryKey = useUserQueryKey();
  return useQuery({
    queryKey: userQueryKey("temporary-episode-detail", target.tmdbID, seasonNumber, episodeNumber),
    enabled: target.mediaType === "tv" && Boolean(target.episodeID) && seasonNumber !== undefined && episodeNumber !== undefined,
    queryFn: () => api.get<TemporaryEpisodeDetails>(`/api/v1/discover/shows/${target.tmdbID}/seasons/${seasonNumber}/episodes/${episodeNumber}`, "Could not load episode details."),
  });
}

export function useEpisodeRatingQuery(episodeID: string | undefined) {
  const userQueryKey = useUserQueryKey();
  return useQuery({
    queryKey: userQueryKey("episode-rating", episodeID),
    enabled: Boolean(episodeID),
    queryFn: () => api.get<EpisodeRating>(`/api/v1/episodes/${encodeURIComponent(episodeID!)}/rating`, "Could not load episode rating."),
  });
}
