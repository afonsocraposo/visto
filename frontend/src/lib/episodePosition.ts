export function episodePosition(label?: string): {
  seasonNumber?: number;
  episodeNumber?: number;
} {
  const match = /^S(\d+)E(\d+)$/i.exec(label ?? "");
  return match ? { seasonNumber: Number(match[1]), episodeNumber: Number(match[2]) } : {};
}

/** "S2 E7", or "Special 3" for season 0. */
export function episodeCode(episode: { season_number: number; episode_number: number }) {
  return episode.season_number === 0
    ? `Special ${episode.episode_number}`
    : `S${episode.season_number} E${episode.episode_number}`;
}

/** "S02E06" history labels as "S2 E6". */
export function episodeLabelCode(label?: string) {
  const { seasonNumber, episodeNumber } = episodePosition(label);
  return seasonNumber === undefined || episodeNumber === undefined
    ? label
    : episodeCode({ season_number: seasonNumber, episode_number: episodeNumber });
}
