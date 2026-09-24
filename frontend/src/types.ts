export type Theme = "system" | "light" | "dark";
export type Tab = "watch" | "search" | "feed" | "library";

export type User = {
  id: string;
  username: string;
  display_name: string;
  role: "admin" | "user";
};

export type PersonalAPIToken = {
  id: string;
  name: string;
  created_at: string;
  last_used_at?: string;
  expires_at?: string;
};
export type IssuedPersonalAPIToken = PersonalAPIToken & { token: string };
export type ConnectedApp = {
  client_id: string;
  client_name: string;
  scopes: string[];
  connected_at?: string;
  last_used_at?: string;
  expires_at: string;
};

export type SearchMedia = {
  tmdb_id: number;
  type: "movie" | "tv";
  title: string;
  original_title: string;
  overview: string;
  release_date: string;
  poster_path: string;
  original_language: string;
  backdrop_path?: string;
};
export type TrendingResponse = { tv: SearchMedia[]; movies: SearchMedia[] };

export type LibraryEntry = {
  item: { media_id: string; status: string; rating: number | null; notifications_enabled: boolean; updated_at: string };
  media: SearchMedia & { id: string };
  completed: boolean;
  progress?: { watched_episodes: number; total_episodes: number };
};

export type FeedItem = {
  id: string;
  display_name: string;
  kind: "watch" | "rewatch" | "rating" | "bulk_watch";
  title: string;
  rating?: number;
  count?: number;
  season_number?: number;
  episode_number?: number;
};

export type Episode = {
  id: string;
  season_number: number;
  episode_number: number;
  air_date: string | null;
};

export type ContinueEntry = {
  show_id: string;
  title: string;
  poster_path: string;
  kind: "continue" | "start";
  next_episode?: Episode;
  next_episode_name?: string;
  next_episode_still_path?: string;
  remaining_episodes: number;
  missing_prior_episodes?: Episode[];
};

export type CalendarEntry = { show_id: string; title: string; episode: Episode };
export type ShowEpisodeEntry = { episode: Episode; name: string; overview?: string; runtime?: number; still_path?: string; watched: boolean };
export type EpisodeRating = { episode_id: string; rating: number | null; updated_at?: string };
export type TVCastMember = { id: number; name: string; character: string; profile_path?: string };
export type PersonCredit = {
  tmdb_id: number;
  type: "movie" | "tv";
  title: string;
  original_title?: string;
  character?: string;
  overview?: string;
  release_date?: string;
  poster_path?: string;
  backdrop_path?: string;
  original_language?: string;
  popularity: number;
};
export type PersonDetails = {
  tmdb_id: number;
  name: string;
  biography: string;
  profile_path?: string;
  birthday?: string;
  deathday?: string;
  place_of_birth?: string;
  known_for_department?: string;
  credits: PersonCredit[];
};
export type TVCrewMember = { id: number; name: string; job: string; department: string; profile_path?: string };
export type TemporaryShowDetails = {
  media: SearchMedia & { id: string; status?: string };
  seasons: { tmdb_id: number; season_number: number; name: string; overview?: string; poster_path?: string; air_date?: string; episodes?: ShowEpisodeEntry[] }[];
  cast: TVCastMember[];
};
export type TemporaryMovieDetails = { media: SearchMedia & { id: string; status?: string }; runtime?: number; vote_average?: number; genres: string[]; cast: TVCastMember[] };
export type TemporaryEpisodeDetails = { name: string; overview?: string; air_date?: string; runtime?: number; still_path?: string; vote_average?: number; production_code?: string; guest_stars?: TVCastMember[]; crew?: TVCrewMember[] };
export type ShowProgress = { cursor: Episode | null; is_caught_up: boolean };

export type MediaDetailTarget = {
  mediaType: "movie" | "tv";
  tmdbID: number;
  mediaID?: string;
  episodeID?: string;
  episode?: Episode;
  seasonNumber?: number;
  seed?: SearchMedia;
};

export type HistoryEntry = {
  play: {
    id: string;
    media_id: string | null;
    episode_id: string | null;
    watched_at: string;
  };
  title: string;
  episode_label?: string;
  episode_name?: string;
  artwork_path?: string;
};

export type LibraryStatus = "watching" | "completed" | "watchlist" | "paused" | "dropped";
