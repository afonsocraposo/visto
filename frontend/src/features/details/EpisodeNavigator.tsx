import { IconChevronLeft, IconChevronRight } from "@tabler/icons-react";
import type { Episode, ShowEpisodeEntry } from "../../types";
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
}: {
  current: Episode;
  previous: ShowEpisodeEntry | null;
  next: ShowEpisodeEntry | null;
}) {
  const navigate = useEpisodeNavigate();
  const side = (entry: ShowEpisodeEntry | null, direction: "previous" | "next") => {
    const label = direction === "previous" ? "Previous episode" : "Next episode";
    const Icon = direction === "previous" ? IconChevronLeft : IconChevronRight;
    return (
      <button
        type="button"
        className={`episode-nav-step is-${direction}`}
        disabled={!entry}
        aria-label={
          entry
            ? `${label}: ${episodeCode(entry.episode)}${entry.name ? `, ${entry.name}` : ""}`
            : `No ${label.toLowerCase()}`
        }
        onClick={() => navigate(direction)}
      >
        <Icon className="episode-nav-chevron" size={22} stroke={2} aria-hidden="true" />
        {entry && (
          <span className="episode-nav-neighbour" aria-hidden="true">
            <span className="episode-nav-neighbour-code">{episodeCode(entry.episode)}</span>
            {entry.name && <span className="episode-nav-neighbour-name">{entry.name}</span>}
          </span>
        )}
      </button>
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
