import type { ReactNode } from "react";
import { DetailLink } from "../../components/DetailLink";
import { FadeImage } from "../../components/FadeImage";
import type { MediaDetailTarget } from "../../types";

/**
 * The card shared by "Up next" rows and the recent watch history rows. The episode title is the
 * main control and stretches over the whole card; the show name and the trailing action are
 * separate controls layered above it, so nothing interactive is nested.
 */
export function WatchRowCard({
  className = "",
  show,
  showTarget,
  showBadge = false,
  title,
  meta,
  art,
  titleTarget,
  onOpenDetail,
  openLabel,
  onAnimationEnd,
  trailing,
}: {
  className?: string;
  /** Context line: the show (or "Movie"). */
  show: string;
  showTarget?: MediaDetailTarget;
  showBadge?: boolean;
  /** Main content: the episode title (or the movie title). */
  title: string;
  meta?: ReactNode;
  art?: string | null;
  titleTarget?: MediaDetailTarget;
  onOpenDetail?: (target: MediaDetailTarget) => void;
  openLabel?: string;
  onAnimationEnd?: (event: React.AnimationEvent<HTMLElement>) => void;
  trailing?: ReactNode;
}) {
  const showContent = showBadge ? <span className="watch-row-show-badge">{show}</span> : show;
  return (
    <article className={`watch-row ${className}`.trim()} onAnimationEnd={onAnimationEnd}>
      <div className="watch-row-art" aria-hidden="true">
        {art ? (
          <FadeImage src={art} alt="" />
        ) : (
          <div className="artwork-fallback">{show.slice(0, 1)}</div>
        )}
      </div>
      <div className="watch-row-content">
        {showTarget ? (
          <DetailLink
            to={showTarget}
            onOpen={onOpenDetail}
            className={showBadge ? "watch-row-show has-badge" : "watch-row-show"}
          >
            {showContent}
          </DetailLink>
        ) : (
          <span className="watch-row-show">{showContent}</span>
        )}
        {titleTarget ? (
          <DetailLink
            to={titleTarget}
            onOpen={onOpenDetail}
            className="watch-row-title watch-row-open"
            aria-label={openLabel}
          >
            {title}
          </DetailLink>
        ) : (
          <span className="watch-row-title">{title}</span>
        )}
        {meta && <span className="watch-row-meta">{meta}</span>}
      </div>
      {trailing}
    </article>
  );
}
