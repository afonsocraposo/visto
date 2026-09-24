import type { ShowEpisodeEntry } from "../../types";

export function findMissingPriorEpisodes(
  entries: ShowEpisodeEntry[],
  target: ShowEpisodeEntry,
  today: string,
): ShowEpisodeEntry[] {
  return entries.filter(entry => {
    const episode = entry.episode;
    const comesBefore = episode.season_number < target.episode.season_number
      || (episode.season_number === target.episode.season_number && episode.episode_number < target.episode.episode_number);
    const released = !episode.air_date || episode.air_date <= today;
    return comesBefore && episode.season_number > 0 && released && !entry.watched;
  });
}
