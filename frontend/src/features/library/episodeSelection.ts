import type { ShowEpisodeEntry } from "../../types";

function before(a: ShowEpisodeEntry, b: ShowEpisodeEntry): boolean {
  return (
    a.episode.season_number < b.episode.season_number ||
    (a.episode.season_number === b.episode.season_number &&
      a.episode.episode_number < b.episode.episode_number)
  );
}

export function findMissingPriorEpisodes(
  entries: ShowEpisodeEntry[],
  target: ShowEpisodeEntry,
  today: string,
): ShowEpisodeEntry[] {
  // Only the gap since the last watched episode: someone starting mid-show shouldn't be asked about earlier seasons.
  const lastWatched = entries
    .filter((entry) => entry.watched && before(entry, target))
    .reduce<ShowEpisodeEntry | undefined>((latest, entry) => (!latest || before(latest, entry) ? entry : latest), undefined);
  return entries.filter((entry) => {
    const episode = entry.episode;
    const released = !episode.air_date || episode.air_date <= today;
    return (
      before(entry, target) &&
      (!lastWatched || before(lastWatched, entry)) &&
      episode.season_number > 0 &&
      released &&
      !entry.watched
    );
  });
}
