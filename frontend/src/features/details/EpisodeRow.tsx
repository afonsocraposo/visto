import { useEffect, useRef, useState } from "react";
import { ActionIcon, Image, Menu } from "@mantine/core";
import { IconCheck, IconDots, IconRefresh } from "@tabler/icons-react";
import { backdropURL } from "../../lib/artwork";
import { episodeCode } from "../../lib/episodePosition";
import { formatEpisodeAirDate, isUnreleased } from "../../lib/airDate";
import type { ShowEpisodeEntry } from "../../types";

/**
 * One episode in a season list: "4 · Title", then "S3 E4 · Mar 18", then one line of overview.
 * The title opens the episode (its hit area covers the row); the watched check is a separate
 * control. Becoming watched pops the check and briefly highlights the row in place.
 */
export function EpisodeRow({
  entry,
  onOpen,
  onToggleWatched,
  onRewatch,
  disabled,
}: {
  entry: ShowEpisodeEntry;
  onOpen: () => void;
  onToggleWatched: () => void;
  onRewatch: () => void;
  disabled: boolean;
}) {
  const { episode } = entry;
  const justWatched = useJustBecameTrue(entry.watched);
  const release = formatEpisodeAirDate(episode.air_date);
  const future = isUnreleased(episode.air_date);
  const still = backdropURL(entry.still_path, "w780");
  const name = entry.name || `Episode ${episode.episode_number}`;

  return (
    <article
      className={`episode-row${entry.watched ? " is-watched" : ""}${future ? " is-future" : ""}${justWatched ? " is-just-watched" : ""}`}
    >
      <div className="episode-art" aria-hidden="true">
        {still ? (
          <Image src={still} alt="" loading="lazy" />
        ) : (
          <div className="artwork-fallback">{episode.episode_number}</div>
        )}
      </div>
      <div className="episode-row-copy">
        <button type="button" className="episode-row-title episode-row-open" onClick={onOpen}>
          <span className="episode-row-number">{episode.episode_number}</span>
          {" · "}
          {name}
        </button>
        <p className="episode-row-meta">
          <span>{episodeCode(episode)}</span>
          {release && <span className={future ? "is-upcoming" : undefined}>{release}</span>}
        </p>
        {entry.overview && <p className="episode-description">{entry.overview}</p>}
      </div>
      <div className="episode-row-controls">
        <button
          type="button"
          role="checkbox"
          className="episode-check"
          aria-checked={entry.watched}
          aria-label={`Episode ${episode.episode_number} watched`}
          title={`Mark episode ${episode.episode_number} ${entry.watched ? "unwatched" : "watched"}`}
          disabled={disabled}
          onClick={onToggleWatched}
        >
          <span className="episode-check-mark">
            <IconCheck size={16} stroke={3} />
          </span>
        </button>
        {entry.watched && (
          <Menu withinPortal position="bottom-end">
            <Menu.Target>
              <ActionIcon
                className="episode-row-more"
                aria-label={`More actions for episode ${episode.episode_number}`}
                size={36}
                variant="subtle"
                color="gray"
              >
                <IconDots size={16} />
              </ActionIcon>
            </Menu.Target>
            <Menu.Dropdown>
              <Menu.Item leftSection={<IconRefresh size={15} />} onClick={onRewatch}>
                Rewatch episode
              </Menu.Item>
            </Menu.Dropdown>
          </Menu>
        )}
      </div>
    </article>
  );
}

/** True for a moment after `value` flips from false to true (not on first render). */
function useJustBecameTrue(value: boolean, ms = 700) {
  const previous = useRef(value);
  const [active, setActive] = useState(false);
  useEffect(() => {
    if (value && !previous.current) {
      setActive(true);
      const timer = window.setTimeout(() => setActive(false), ms);
      previous.current = value;
      return () => window.clearTimeout(timer);
    }
    previous.current = value;
  }, [value, ms]);
  return active;
}
