import type { ReactNode } from "react";
import { Avatar, Group, Image, Paper, Text } from "@mantine/core";
import { IconDeviceTv, IconMovie } from "@tabler/icons-react";
import { RatingStars } from "../../components/RatingStars";
import { ActivityTime } from "../../components/ActivityTime";
import { backdropURL, posterURL } from "../../lib/artwork";

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
  onOpenDetail?: () => void;
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
  onOpenDetail,
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
      role={onOpenDetail ? "link" : undefined}
      tabIndex={onOpenDetail ? 0 : undefined}
      aria-label={onOpenDetail ? `Open details for ${title}` : undefined}
      onClick={(event) => {
        if (!(event.target as HTMLElement).closest("button, input, form, [role='menuitem']")) {
          onOpenDetail?.();
        }
      }}
      onKeyDown={(event) => {
        if (event.target === event.currentTarget && (event.key === "Enter" || event.key === " ")) {
          event.preventDefault();
          onOpenDetail?.();
        }
      }}
    >
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
