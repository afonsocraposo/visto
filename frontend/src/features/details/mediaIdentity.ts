import type { SearchMedia } from "../../types";

export function resolveMediaID(targetMediaID: string | undefined, savedMediaID: string | undefined, media: SearchMedia | undefined): string | undefined {
  return targetMediaID || savedMediaID || (media ? `${media.type}:${media.tmdb_id}` : undefined);
}
