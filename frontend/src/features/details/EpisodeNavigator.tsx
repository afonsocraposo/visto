import { IconChevronLeft, IconChevronRight } from "@tabler/icons-react";
import { DetailLink } from "../../components/DetailLink";
import type { Episode, MediaDetailTarget, ShowEpisodeEntry } from "../../types";
import { useEpisodeNavigate } from "./EpisodeTransition";
import { episodeCode } from "../../lib/episodePosition";

export { episodeCode };

/**
 * The current episode sits in the middle with previous/next chevrons around it. On wide screens
 * the neighbours' codes and titles are shown as well.
 */
export function EpisodeNavigator({
  current,
  previous,
  next,
  showTarget,
}: {
  current: Episode;
  previous: ShowEpisodeEntry | null;
  next: ShowEpisodeEntry | null;
  showTarget: MediaDetailTarget;
}) {
  const navigate = useEpisodeNavigate();
  const side = (entry: ShowEpisodeEntry | null, direction: "previous" | "next") => {
    const label = direction === "previous" ? "Previous episode" : "Next episode";
    const Icon = direction === "previous" ? IconChevronLeft : IconChevronRight;
    const content = (
      <>
        <Icon className="episode-nav-chevron" size={22} stroke={2} aria-hidden="true" />
        {entry && (
          <span className="episode-nav-neighbour" aria-hidden="true">
            <span className="episode-nav-neighbour-code">{episodeCode(entry.episode)}</span>
            {entry.name && <span className="episode-nav-neighbour-name">{entry.name}</span>}
          </span>
        )}
      </>
    );
    const className = `episode-nav-step is-${direction}`;
    if (!entry)
      return (
        <button
          type="button"
          className={className}
          disabled
          aria-label={`No ${label.toLowerCase()}`}
        >
          {content}
        </button>
      );
    return (
      <DetailLink
        to={{
          ...showTarget,
          episodeID: entry.episode.id,
          episode: entry.episode,
          seasonNumber: entry.episode.season_number,
          episodeNumber: entry.episode.episode_number,
        }}
        onOpen={() => navigate(direction)}
        className={className}
        aria-label={`${label}: ${episodeCode(entry.episode)}${entry.name ? `, ${entry.name}` : ""}`}
      >
        {content}
      </DetailLink>
    );
  };

  return (
    <nav className="episode-nav" aria-label="Episode navigation">
      {side(previous, "previous")}
      <span className="episode-nav-current" aria-current="page">
        {episodeCode(current)}
      </span>
      {side(next, "next")}
    </nav>
  );
}
