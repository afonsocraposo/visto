import type { ReactNode } from "react";
import { FadeImage } from "../../components/FadeImage";

/**
 * The card shared by "Up next" rows and the recent watch history rows. The episode title is the
 * main control and stretches over the whole card; the show name and the trailing action are
 * separate controls layered above it, so nothing interactive is nested.
 */
export function WatchRowCard({
  className = "",
  show,
  onOpenShow,
  title,
  meta,
  art,
  onOpen,
  openLabel,
  onAnimationEnd,
  trailing,
}: {
  className?: string;
  /** Context line: the show (or "Movie"). */
  show: string;
  onOpenShow?: () => void;
  /** Main content: the episode title (or the movie title). */
  title: string;
  meta?: ReactNode;
  art?: string | null;
  onOpen?: () => void;
  openLabel?: string;
  onAnimationEnd?: (event: React.AnimationEvent<HTMLElement>) => void;
  trailing?: ReactNode;
}) {
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
        {onOpenShow ? (
          <button type="button" className="watch-row-show" onClick={onOpenShow}>
            {show}
          </button>
        ) : (
          <span className="watch-row-show">{show}</span>
        )}
        {onOpen ? (
          <button
            type="button"
            className="watch-row-title watch-row-open"
            aria-label={openLabel}
            onClick={onOpen}
          >
            {title}
          </button>
        ) : (
          <span className="watch-row-title">{title}</span>
        )}
        {meta && <span className="watch-row-meta">{meta}</span>}
      </div>
      {trailing}
    </article>
  );
}
