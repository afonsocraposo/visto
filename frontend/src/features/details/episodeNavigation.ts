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
