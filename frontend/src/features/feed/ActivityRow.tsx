import type { ReactNode } from "react";
import { Avatar, Group, Image, Paper, Text } from "@mantine/core";
import { IconDeviceTv, IconMovie } from "@tabler/icons-react";
import { RatingStars } from "../../components/RatingStars";
import { backdropURL, posterURL } from "../../lib/artwork";
import { formatActivityTime } from "../../lib/time";

type Props = {
  title: string;
  mediaType?: "movie" | "tv";
  artworkPath?: string;
  actor?: string;
  action: string;
  episodeLabel?: string;
  episodeName?: string;
  rating?: number;
  occurredAt: string | Date;
  exactTime?: ReactNode;
  actions?: ReactNode;
  children?: ReactNode;
};

export function ActivityRow({
  title,
  mediaType,
  artworkPath,
  actor,
  action,
  episodeLabel,
  episodeName,
  rating,
  occurredAt,
  exactTime,
  actions,
  children,
}: Props) {
  const date = occurredAt instanceof Date ? occurredAt : new Date(occurredAt);
  const art =
    episodeLabel && mediaType === "tv"
      ? backdropURL(artworkPath, "w780")
      : posterURL(artworkPath, "w342");

  return (
    <Paper className="activity-row" component="article" withBorder>
      <div className="activity-row-art" aria-hidden="true">
        {art ? (
          <Image src={art} alt="" loading="lazy" />
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
            {actor && (
              <Avatar className="activity-avatar" size={30} aria-hidden="true">
                {actor.trim().slice(0, 1).toLocaleUpperCase()}
              </Avatar>
            )}
            <Text size="sm" c="dimmed" lineClamp={1}>
              <Text component="span" fw={700} c="var(--mantine-color-text)">
                {actor || "You"}
              </Text>{" "}
              {action}
            </Text>
          </Group>
          {actions}
        </Group>
        <Text className="activity-row-title" fw={750} lineClamp={1}>
          {title}
        </Text>
        {(episodeLabel || episodeName) && (
          <Group className="activity-row-episode" gap="xs" wrap="wrap">
            {episodeLabel && <span className="activity-episode-badge">{episodeLabel}</span>}
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
            {formatActivityTime(date)}
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
