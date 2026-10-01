import type { ReactNode } from "react";
import { FadeImage } from "../../components/FadeImage";
import { Avatar, Group, Paper, Text } from "@mantine/core";
import { IconDeviceTv, IconMovie } from "@tabler/icons-react";
import { RatingStars } from "../../components/RatingStars";
import { ActivityTime } from "../../components/ActivityTime";
import { backdropURL, posterURL } from "../../lib/artwork";
import { episodeLabelCode } from "../../lib/episodePosition";

type Props = {
  title: string;
  mediaType?: "movie" | "tv";
  artworkPath?: string;
  actor?: string;
  onOpenActor?: () => void;
  action: string;
  episodeLabel?: string;
  episodeName?: string;
  rating?: number;
  occurredAt: string | Date;
  exactTime?: ReactNode;
  actions?: ReactNode;
  children?: ReactNode;
  onOpenDetail?: () => void;
  onOpenShow?: () => void;
};

export function ActivityRow({
  title,
  mediaType,
  artworkPath,
  actor,
  onOpenActor,
  action,
  episodeLabel,
  episodeName,
  rating,
  occurredAt,
  exactTime,
  actions,
  children,
  onOpenDetail,
  onOpenShow,
}: Props) {
  const art =
    episodeLabel && mediaType === "tv"
      ? backdropURL(artworkPath, "w780")
      : posterURL(artworkPath, "w342");

  return (
    <Paper
      className={`activity-row${onOpenDetail ? " activity-row-clickable" : ""}`}
      component="article"
      withBorder
    >
      {onOpenDetail && (
        // Covers the card; the actor, show and menu controls are layered above it.
        <button type="button" className="activity-row-open" onClick={onOpenDetail}>
          <span className="visually-hidden">Open details for {title}</span>
        </button>
      )}
      <div className="activity-row-art" aria-hidden="true">
        {art ? (
          <FadeImage src={art} alt="" />
        ) : mediaType === "tv" ? (
          <IconDeviceTv size={24} />
        ) : (
          <IconMovie size={24} />
        )}
      </div>
      <div className="activity-row-main">
        <Group
          className="activity-row-heading"
          justify="space-between"
          align="center"
          gap="sm"
          wrap="nowrap"
        >
          <Group gap="xs" wrap="nowrap" className="activity-row-byline">
            {onOpenActor ? (
              <button
                type="button"
                className="activity-actor-button"
                onClick={onOpenActor}
                aria-label={`Open ${actor || "your"} profile`}
              >
                <Avatar className="activity-avatar" size={30} aria-hidden="true">
                  {(actor || "You").trim().slice(0, 1).toLocaleUpperCase()}
                </Avatar>
                <span>{actor || "You"}</span>
              </button>
            ) : (
              <>
                {actor && (
                  <Avatar className="activity-avatar" size={30} aria-hidden="true">
                    {actor.trim().slice(0, 1).toLocaleUpperCase()}
                  </Avatar>
                )}
                <Text component="span" size="sm" fw={700}>
                  {actor || "You"}
                </Text>
              </>
            )}
            <Text component="span" size="sm" c="dimmed">
              {action}
            </Text>
          </Group>
          {actions}
        </Group>
        {onOpenShow ? (
          <button
            type="button"
            className="activity-row-title activity-show-link"
            onClick={(event) => {
              event.stopPropagation();
              onOpenShow();
            }}
          >
            {title}
          </button>
        ) : (
          <Text className="activity-row-title" fw={750} lineClamp={1}>
            {title}
          </Text>
        )}
        {(episodeLabel || episodeName) && (
          <Group className="activity-row-episode" gap="xs" wrap="wrap">
            {episodeLabel && (
              <span className="activity-episode-badge">{episodeLabelCode(episodeLabel)}</span>
            )}
            {episodeName && (
              <Text size="sm" c="dimmed" lineClamp={1}>
                {episodeName}
              </Text>
            )}
          </Group>
        )}
        {rating != null && (
          <RatingStars
            value={rating}
            label={`Rating: ${rating} out of 5 stars`}
            readOnly
            size="sm"
          />
        )}
        <Group className="activity-row-meta" gap="xs" wrap="wrap">
          <Text size="xs" c="dimmed">
            <ActivityTime value={occurredAt} />
          </Text>
          {exactTime && (
            <Text className="activity-row-exact-time" size="xs" c="dimmed">
              {exactTime}
            </Text>
          )}
        </Group>
        {children}
      </div>
    </Paper>
  );
}
