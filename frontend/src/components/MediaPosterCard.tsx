import { Paper, Text } from "@mantine/core";
import type { MediaDetailTarget, SearchMedia } from "../types";
import { posterURL } from "../lib/artwork";

type Props = {
  media: SearchMedia;
  onOpenDetail?: (target: MediaDetailTarget) => void;
  target?: MediaDetailTarget;
};

export function MediaPosterCard({ media, onOpenDetail, target }: Props) {
  const art = posterURL(media.poster_path, "w342");
  return <Paper className="media-poster-card" component="button" type="button" withBorder style={art ? { backgroundImage: `url(${art})` } : undefined} onClick={() => onOpenDetail?.(target ?? { mediaType: media.type, tmdbID: media.tmdb_id, seed: media })} aria-label={`Open details for ${media.title}`}>
    {!art && <div className="artwork-fallback">{media.title.slice(0, 1)}</div>}
    <span className="media-poster-card-scrim" aria-hidden="true" />
    <span className="media-poster-card-title"><Text component="span" fw={750} lineClamp={2}>{media.title}</Text><Text component="span" size="xs">{media.release_date ? media.release_date.slice(0, 4) : ""}</Text></span>
  </Paper>;
}
