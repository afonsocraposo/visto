export function episodePosition(label?: string): {
  seasonNumber?: number;
  episodeNumber?: number;
} {
  const match = /^S(\d+)E(\d+)$/i.exec(label ?? "");
  return match ? { seasonNumber: Number(match[1]), episodeNumber: Number(match[2]) } : {};
}
