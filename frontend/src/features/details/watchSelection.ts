import type { ShowEpisodeEntry } from "../../types";

export function selectUnwatchedEpisodes(
  episodes: ShowEpisodeEntry[],
  seasonNumbers: number[],
  today: string,
): string[] {
  return episodes
    .filter(entry => seasonNumbers.includes(entry.episode.season_number)
      && !entry.watched
      && (!entry.episode.air_date || entry.episode.air_date <= today))
    .map(entry => entry.episode.id);
}

export function regularSeasonsThrough(seasonNumbers: number[], selectedSeason: number): number[] {
  return [...new Set(seasonNumbers.filter(number => number > 0 && number <= selectedSeason))].sort((a, b) => a - b);
}
