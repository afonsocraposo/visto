import type { SearchMedia, TrendingResponse } from "../../types";

export const loginBackdropCacheKey = "visto:login-backdrop:v2";
export const loginBackdropTTL = 60_000;

export type LoginBackdrop = {
  title: string;
  type: SearchMedia["type"];
  tmdb_id: number;
  backdrop_path: string;
  expiresAt: number;
};

export function readLoginBackdrop(storage: Pick<Storage, "getItem">, now: number): LoginBackdrop | null {
  try {
    const raw = storage.getItem(loginBackdropCacheKey);
    if (!raw) return null;
    const value: unknown = JSON.parse(raw);
    if (!value || typeof value !== "object") return null;
    const item = value as Partial<LoginBackdrop>;
    if (typeof item.expiresAt !== "number" || item.expiresAt <= now || typeof item.title !== "string"
      || (item.type !== "tv" && item.type !== "movie") || typeof item.backdrop_path !== "string"
      || !Number.isSafeInteger(item.tmdb_id) || item.tmdb_id! <= 0
      || !/^\/[a-zA-Z0-9_-]+\.(jpg|jpeg|png|webp)$/.test(item.backdrop_path)) return null;
    return item as LoginBackdrop;
  } catch {
    return null;
  }
}

export function pickLoginBackdrop(trending: TrendingResponse, now: number, random: () => number = Math.random): LoginBackdrop | null {
  const candidates = [...trending.tv, ...trending.movies].filter(item => item.backdrop_path && Number.isSafeInteger(item.tmdb_id) && item.tmdb_id > 0);
  if (!candidates.length) return null;
  const index = Math.min(candidates.length - 1, Math.max(0, Math.floor(random() * candidates.length)));
  const chosen = candidates[index];
  return { title: chosen.title, type: chosen.type, tmdb_id: chosen.tmdb_id, backdrop_path: chosen.backdrop_path!, expiresAt: now + loginBackdropTTL };
}

export function tmdbTitleURL(media: Pick<LoginBackdrop, "type" | "tmdb_id">): string {
  return `https://www.themoviedb.org/${media.type}/${media.tmdb_id}`;
}

export function saveLoginBackdrop(storage: Pick<Storage, "setItem">, selection: LoginBackdrop): void {
  try {
    storage.setItem(loginBackdropCacheKey, JSON.stringify(selection));
  } catch {
    // The backdrop is optional when storage is unavailable.
  }
}
