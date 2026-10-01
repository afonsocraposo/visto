import { Text } from "@mantine/core";
import type { MediaDetailTarget, SearchMedia } from "../types";
import { posterURL } from "../lib/artwork";
import { DetailLink } from "./DetailLink";
import { FadeImage } from "./FadeImage";

type Props = {
  media: SearchMedia;
  onOpenDetail?: (target: MediaDetailTarget) => void;
  target?: MediaDetailTarget;
  progress?: { value: number; label: string };
  subtitle?: string;
  /**
   * "overlay" sets the title on the artwork for dense, cinematic grids (Library, related);
   * "caption" puts it below the poster for legibility (Discover, filmography).
   */
  variant?: "overlay" | "caption";
  /** Load immediately instead of lazily (posters in the first viewport). */
  eager?: boolean;
};

export function MediaPosterCard({
  media,
  onOpenDetail,
  target,
  progress,
  subtitle,
  variant = "overlay",
  eager = false,
}: Props) {
  const art = posterURL(media.poster_path, "w342");
  const year = media.release_date ? media.release_date.slice(0, 4) : "";
  const destination = target ?? { mediaType: media.type, tmdbID: media.tmdb_id, seed: media };
  const titleBlock = (
    <>
      <Text
        component="span"
        className="media-poster-card-name"
        fw={variant === "overlay" ? 750 : 650}
        lineClamp={2}
      >
        {media.title}
      </Text>
      {year && (
        <Text component="span" className="media-poster-card-year" size="xs">
          {year}
        </Text>
      )}
      {subtitle && (
        <Text component="span" className="media-poster-card-subtitle" size="xs" lineClamp={1}>
          {subtitle}
        </Text>
      )}
    </>
  );

  return (
    <article className={`media-poster-card is-${variant}`}>
      <DetailLink
        className="media-poster-card-link"
        to={destination}
        onOpen={onOpenDetail}
        aria-label={`Open details for ${media.title}`}
      >
        <span className="media-poster-card-art">
          {art ? (
            <FadeImage src={art} alt="" eager={eager} />
          ) : (
            <span className="artwork-fallback">{media.title.slice(0, 1)}</span>
          )}
          {variant === "overlay" && (
            <>
              <span className="media-poster-card-scrim" aria-hidden="true" />
              <span className="media-poster-card-title">{titleBlock}</span>
            </>
          )}
          {progress && (
            <span
              className="media-poster-card-progress"
              role="progressbar"
              aria-label={progress.label}
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={progress.value}
            >
              <span style={{ width: `${progress.value}%` }} />
            </span>
          )}
        </span>
        {variant === "caption" && <span className="media-poster-card-caption">{titleBlock}</span>}
      </DetailLink>
    </article>
  );
}
