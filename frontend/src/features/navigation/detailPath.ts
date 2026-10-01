import type { MediaDetailTarget } from "../../types";

/** The canonical path of a media or episode page (without the in-app return parameters). */
export function detailPath(
  target: Pick<MediaDetailTarget, "mediaType" | "tmdbID" | "seasonNumber" | "episodeNumber">,
): string {
  if (
    target.mediaType === "tv" &&
    target.seasonNumber !== undefined &&
    target.episodeNumber !== undefined
  )
    return `/shows/${target.tmdbID}/season/${target.seasonNumber}/episode/${target.episodeNumber}`;
  return `/media/${target.mediaType}/${target.tmdbID}`;
}
