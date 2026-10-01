import type { Episode, ShowEpisodeEntry } from "../../types";

export function getAdjacentEpisodes(
  currentEpisode: Episode | undefined,
  episodes: ShowEpisodeEntry[],
) {
  if (!currentEpisode) return { previousEpisode: null, nextEpisode: null };

  const sequence = episodes
    .filter((entry) =>
      currentEpisode.season_number === 0
        ? entry.episode.season_number === 0
        : entry.episode.season_number > 0,
    )
    .sort(
      (left, right) =>
        left.episode.season_number - right.episode.season_number ||
        left.episode.episode_number - right.episode.episode_number,
    );
  const currentIndex = sequence.findIndex(
    (entry) =>
      entry.episode.id === currentEpisode.id ||
      (entry.episode.season_number === currentEpisode.season_number &&
        entry.episode.episode_number === currentEpisode.episode_number),
  );

  return {
    previousEpisode: currentIndex > 0 ? sequence[currentIndex - 1] : null,
    nextEpisode: currentIndex >= 0 ? (sequence[currentIndex + 1] ?? null) : null,
  };
}

/** Touches starting this close to a screen edge belong to the OS (iOS back swipe, Android gestures). */
const EDGE_GUARD = 32;
const MIN_DISTANCE = 64;

export type SwipeDirection = "previous" | "next";

/** How far (px) the content may follow the finger, and how much it fades at most. */
const FOLLOW_MAX = 28;
const FOLLOW_FADE = 0.12;

/**
 * While dragging, the episode follows the finger at a fraction of the distance (an 80px drag
 * moves it ~24px) and dims slightly. With no episode that way it resists much harder.
 */
export function dragFollow(dx: number, hasTarget: boolean): { shift: number; opacity: number } {
  const ratio = hasTarget ? 0.3 : 0.08;
  const shift = Math.sign(dx) * Math.min(Math.abs(dx) * ratio, FOLLOW_MAX);
  const opacity = hasTarget ? 1 - Math.min(Math.abs(dx) / MIN_DISTANCE / 1.25, 1) * FOLLOW_FADE : 1;
  return { shift: shift || 0, opacity };
}

export function swipeDirection(
  start: [number, number],
  end: [number, number],
  viewportWidth: number,
): SwipeDirection | null {
  if (start[0] < EDGE_GUARD || start[0] > viewportWidth - EDGE_GUARD) return null;
  const dx = end[0] - start[0];
  const dy = end[1] - start[1];
  if (Math.abs(dx) < MIN_DISTANCE || Math.abs(dx) < Math.abs(dy) * 1.5) return null;
  return dx > 0 ? "previous" : "next";
}
