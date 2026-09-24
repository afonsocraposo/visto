export type Theme = "system" | "light" | "dark";
export type Tab = "watch" | "search" | "feed" | "library";

export type User = {
  id: string;
  username: string;
  display_name: string;
  role: "admin" | "user";
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
};

export type LibraryEntry = {
  item: { media_id: string; status: string; rating: number | null };
  media: SearchMedia & { id: string };
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
  missing_prior_episodes?: Episode[];
};

export type CalendarEntry = { show_id: string; title: string; episode: Episode };

export type HistoryEntry = {
  play: {
    id: string;
    media_id: string | null;
    episode_id: string | null;
    watched_at: string;
  };
  title: string;
  episode_label?: string;
};
