import { Badge, Button, Group, Select, Switch, Text } from "@mantine/core";
import { IconCheck, IconEye, IconRefresh } from "@tabler/icons-react";
import { RatingStars } from "../../components/RatingStars";
import { MediaQuickActions } from "../../components/MediaQuickActions";
import type { SearchMedia, ShowEpisodeEntry } from "../../types";

type ActionMutation<T> = { isPending: boolean; mutate: (value: T) => void };

type EpisodeActionsProps = {
  entry: ShowEpisodeEntry;
  canRate: boolean;
  rating?: number | null;
  onRate: (rating: number | null) => void;
  onWatch: () => void;
  onRewatch: () => void;
  onUnwatch: () => void;
  pending: boolean;
};

export function EpisodeActions({
  entry,
  canRate,
  rating,
  onRate,
  onWatch,
  onRewatch,
  onUnwatch,
  pending,
}: EpisodeActionsProps) {
  return (
    <Group className="detail-actions" mt="lg">
      <Badge color={entry.watched ? "teal" : "yellow"} variant="light">
        {entry.watched ? "Watched" : "Not watched"}
      </Badge>
      <RatingStars
        value={rating}
        onChange={onRate}
        label="Episode rating"
        disabled={!canRate || pending}
        size="md"
      />
      {!canRate && (
        <Text size="xs" c="dimmed">
          Add to library to rate
        </Text>
      )}
      {entry.watched ? (
        <>
          <Button
            variant="light"
            leftSection={<IconRefresh size={16} />}
            loading={pending}
            onClick={onRewatch}
          >
            Rewatch episode
          </Button>
          <Button
            variant="default"
            leftSection={<IconEye size={16} />}
            loading={pending}
            onClick={onUnwatch}
          >
            Mark unwatched
          </Button>
        </>
      ) : (
        <Button leftSection={<IconEye size={16} />} loading={pending} onClick={onWatch}>
          Mark watched
        </Button>
      )}
    </Group>
  );
}

type MediaActionsProps = {
  media: SearchMedia;
  isSaved: boolean;
  status?: string;
  rating?: number | null;
  notificationsEnabled: boolean;
  add: ActionMutation<"watching" | "watchlist">;
  update: ActionMutation<{ status: string; rating: number | null }>;
  updateNotifications: ActionMutation<boolean>;
  watched: boolean;
  onWatch: () => void;
  onUnwatch: () => void;
  pending: boolean;
};

export function MediaActions({
  media,
  isSaved,
  status,
  rating,
  notificationsEnabled,
  add,
  update,
  updateNotifications,
  watched,
  onWatch,
  onUnwatch,
  pending,
}: MediaActionsProps) {
  if (!isSaved)
    return (
      <Group className="detail-actions" mt="lg">
        <MediaQuickActions
          media={media}
          busy={add.isPending || pending}
          onWatch={() => (media.type === "tv" ? add.mutate("watching") : onWatch())}
          onWatchlist={() => add.mutate("watchlist")}
        />
      </Group>
    );
  const statusOptions: { value: string; label: string; disabled?: boolean }[] = [
    { value: "watchlist", label: "Watchlist" },
    { value: "watching", label: "Watching" },
    ...(media.type === "movie"
      ? []
      : [
          { value: "paused", label: "Paused" },
          { value: "dropped", label: "Dropped" },
        ]),
  ];
  if (media.type === "movie" && status && !statusOptions.some((option) => option.value === status))
    statusOptions.push({
      value: status,
      label: `${status[0].toUpperCase()}${status.slice(1)} (not available for movies)`,
      disabled: true,
    });
  return (
    <Group className="detail-actions" mt="lg">
      <Select
        aria-label="Current list"
        value={status}
        onChange={(value) => value && update.mutate({ status: value, rating: rating ?? null })}
        data={statusOptions}
        w={150}
      />
      <RatingStars
        value={rating}
        onChange={(value) => update.mutate({ status: status!, rating: value })}
        label="Media rating"
        disabled={pending}
        size="md"
      />
      {media.type === "tv" && (
        <Switch
          aria-label="New episode notifications for this show"
          label="Episode alerts"
          checked={notificationsEnabled}
          disabled={updateNotifications.isPending}
          onChange={(event) => updateNotifications.mutate(event.currentTarget.checked)}
        />
      )}
      {media.type === "movie" &&
        (watched ? (
          <>
            <Button
              variant="light"
              leftSection={<IconRefresh size={16} />}
              loading={pending}
              onClick={onWatch}
            >
              Rewatch movie
            </Button>
            <Button
              variant="default"
              leftSection={<IconEye size={16} />}
              loading={pending}
              onClick={onUnwatch}
            >
              Mark unwatched
            </Button>
          </>
        ) : (
          <Button
            variant="light"
            leftSection={<IconCheck size={16} />}
            loading={pending}
            onClick={onWatch}
          >
            Mark watched
          </Button>
        ))}
    </Group>
  );
}
