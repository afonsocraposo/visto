export function heroArtworkLayers(
  hasSelectedEpisode: boolean,
  episodeArtwork: string | null,
  episodeArtworkPending: boolean,
  fallbackArtwork: string | null,
): string[] {
  if (hasSelectedEpisode && episodeArtworkPending && !episodeArtwork) return [];
  return [...new Set([episodeArtwork, fallbackArtwork].filter((art): art is string => Boolean(art)))];
}
