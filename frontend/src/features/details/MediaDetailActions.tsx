import { ActionIcon, Button, Group, Menu, Select, Text, Tooltip } from "@mantine/core";
import {
  IconBell,
  IconBellOff,
  IconBookmark,
  IconCheck,
  IconChevronDown,
  IconDots,
  IconEye,
  IconEyeOff,
  IconHistory,
  IconEdit,
  IconPlus,
  IconRefresh,
} from "@tabler/icons-react";
import { RatingStars } from "../../components/RatingStars";
import type { SearchMedia, ShowEpisodeEntry } from "../../types";
import { movieReleaseAlertLabel, showMovieReleaseAlert } from "./movieReleaseAlert";

type ActionMutation<T> = { isPending: boolean; variables?: T; mutate: (value: T) => void };
const completionQuestion =
  "Have you watched every regular episode, including future-dated episodes? Specials are excluded.";

type EpisodeActionsProps = {
  entry: ShowEpisodeEntry;
  canRate: boolean;
  rating?: number | null;
  onRate: (rating: number | null) => void;
  onWatch: () => void;
  onRewatch: () => void;
  onUnwatch: () => void;
  actionPending: boolean;
  ratingPending: boolean;
};

export function EpisodeActions({
  entry,
  canRate,
  rating,
  onRate,
  onWatch,
  onRewatch,
  onUnwatch,
  actionPending,
  ratingPending,
}: EpisodeActionsProps) {
  return (
    <Group className="detail-actions" mt="lg" gap="xs">
      <Button
        className="detail-primary-action"
        color={entry.watched ? "teal" : "yellow"}
        variant={entry.watched ? "light" : "filled"}
        leftSection={entry.watched ? <IconCheck size={18} /> : <IconEye size={18} />}
        aria-label={`Mark episode ${entry.episode.episode_number} ${entry.watched ? "unwatched" : "watched"}`}
        onClick={entry.watched ? onUnwatch : onWatch}
        loading={actionPending}
        disabled={actionPending || ratingPending}
      >
        {entry.watched ? "Watched" : "Mark watched"}
      </Button>
      {entry.watched && (
        <Menu withinPortal position="bottom-start">
          <Menu.Target>
            <ActionIcon
              className="detail-overflow-action"
              size={42}
              variant="default"
              aria-label="More episode actions"
              disabled={actionPending || ratingPending}
            >
              <IconDots size={18} />
            </ActionIcon>
          </Menu.Target>
          <Menu.Dropdown>
            <Menu.Item leftSection={<IconRefresh size={16} />} onClick={onRewatch}>
              Mark rewatched
            </Menu.Item>
            <Menu.Item leftSection={<IconEyeOff size={16} />} onClick={onUnwatch}>
              Mark unwatched
            </Menu.Item>
          </Menu.Dropdown>
        </Menu>
      )}
      <div className="detail-rating">
        <RatingStars
          value={rating}
          onChange={onRate}
          label="Episode rating"
          disabled={!canRate || actionPending || ratingPending}
          size="md"
        />
        {!canRate && (
          <Text size="xs" c="dimmed">
            Add to library to rate
          </Text>
        )}
      </div>
    </Group>
  );
}

type MediaActionsProps = {
  media: SearchMedia;
  isSaved: boolean;
  status?: string;
  rating?: number | null;
  notificationsEnabled: boolean;
  add: ActionMutation<"watching" | "watchlist" | "paused" | "dropped" | "completed">;
  update: ActionMutation<{ status: string; rating: number | null; confirm_all_episodes?: boolean }>;
  updateNotifications: ActionMutation<boolean>;
  notificationMode: "episode" | "season" | "off" | "";
  onOpenNotifications: () => void;
  watched: boolean;
  onWatch: () => void;
  onUnwatch: () => void;
  onRemoveWatchlist: () => void;
  onRemoveCurrentList: () => void;
  onViewWatchHistory: () => void;
  onChangeWatchDate: () => void;
  showBulkAction?: "watch" | "unwatch" | null;
  onShowBulkAction: () => void;
  showBulkPending: boolean;
  watchPending: boolean;
  watchlistPending: boolean;
  removePending: boolean;
  disabled: boolean;
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
  notificationMode,
  onOpenNotifications,
  watched,
  onWatch,
  onUnwatch,
  onRemoveWatchlist,
  onRemoveCurrentList,
  onViewWatchHistory,
  onChangeWatchDate,
  showBulkAction,
  onShowBulkAction,
  showBulkPending,
  watchPending,
  watchlistPending,
  removePending,
  disabled,
}: MediaActionsProps) {
  if (media.type === "movie")
    return (
      <Group className="detail-actions detail-icon-actions" mt="lg" gap="xs">
        <Tooltip label={watched ? "Mark unwatched" : "Mark watched"} withArrow>
          <ActionIcon
            size="lg"
            color="yellow"
            variant="filled"
            aria-label={`Mark ${media.title} ${watched ? "unwatched" : "watched"}`}
            onClick={watched ? onUnwatch : onWatch}
            loading={watched ? removePending : watchPending}
            disabled={disabled}
          >
            {watched ? <IconCheck size={18} /> : <IconEye size={18} />}
          </ActionIcon>
        </Tooltip>
        {watched ? (
          <Menu withinPortal position="bottom-start">
            <Menu.Target>
              <ActionIcon
                size="lg"
                variant="default"
                aria-label={`More actions for ${media.title}`}
                disabled={disabled}
              >
                <IconChevronDown size={18} />
              </ActionIcon>
            </Menu.Target>
            <Menu.Dropdown>
              <Menu.Item leftSection={<IconRefresh size={16} />} onClick={onWatch}>
                Mark rewatched
              </Menu.Item>
              <Menu.Item leftSection={<IconHistory size={16} />} onClick={onViewWatchHistory}>
                View watch history
              </Menu.Item>
              <Menu.Item leftSection={<IconEdit size={16} />} onClick={onChangeWatchDate}>
                Change watch date
              </Menu.Item>
            </Menu.Dropdown>
          </Menu>
        ) : (
          <Tooltip label={isSaved ? "Remove from Watchlist" : "Save for later"} withArrow>
            <ActionIcon
              size="lg"
              variant={isSaved ? "light" : "default"}
              aria-label={`${isSaved ? "Remove" : "Save"} ${media.title} ${isSaved ? "from Watchlist" : "for later"}`}
              onClick={isSaved ? onRemoveWatchlist : () => add.mutate("watchlist")}
              loading={
                isSaved
                  ? watchlistPending
                  : watchlistPending || (add.isPending && add.variables === "watchlist")
              }
              disabled={disabled}
            >
              <IconBookmark size={18} />
            </ActionIcon>
          </Tooltip>
        )}
        {isSaved && (
          <RatingStars
            value={rating}
            onChange={(value) => update.mutate({ status: status!, rating: value })}
            label="Media rating"
            disabled={disabled}
            size="md"
          />
        )}
        {isSaved &&
          showMovieReleaseAlert(
            status,
            media.release_date,
            new Date().toISOString().slice(0, 10),
          ) && (
            <Button
              variant={notificationsEnabled ? "filled" : "default"}
              color={notificationsEnabled ? "yellow" : undefined}
              aria-label={movieReleaseAlertLabel(notificationsEnabled)}
              aria-pressed={notificationsEnabled}
              loading={updateNotifications.isPending}
              onClick={() => updateNotifications.mutate(!notificationsEnabled)}
              leftSection={
                notificationsEnabled ? <IconBell size={20} /> : <IconBellOff size={20} />
              }
            >
              {movieReleaseAlertLabel(notificationsEnabled)}
            </Button>
          )}
      </Group>
    );
  const showWatchAction = showBulkAction && (
    <Tooltip
      label={showBulkAction === "watch" ? "Mark show watched" : "Mark show unwatched"}
      withArrow
    >
      <ActionIcon
        className="show-bulk-action"
        size={44}
        color="yellow"
        variant="light"
        aria-label={`Mark ${media.title} ${showBulkAction === "watch" ? "watched" : "unwatched"}`}
        onClick={onShowBulkAction}
        loading={showBulkPending}
        disabled={disabled || add.isPending}
      >
        {showBulkAction === "watch" ? <IconEye size={20} /> : <IconCheck size={20} />}
      </ActionIcon>
    </Tooltip>
  );
  if (!isSaved)
    return (
      <Group className="detail-actions detail-icon-actions" mt="lg" gap="xs">
        <Tooltip label="Add to Watching" withArrow>
          <ActionIcon
            size={44}
            color="yellow"
            variant="filled"
            loading={add.isPending && add.variables === "watching"}
            disabled={disabled || add.isPending}
            aria-label={`Add ${media.title} to Watching`}
            onClick={() => add.mutate("watching")}
          >
            <IconPlus size={20} />
          </ActionIcon>
        </Tooltip>
        <Tooltip label="Watch later" withArrow>
          <ActionIcon
            size={44}
            variant="default"
            loading={add.isPending && add.variables === "watchlist"}
            disabled={disabled || add.isPending}
            aria-label={`Save ${media.title} for later`}
            onClick={() => add.mutate("watchlist")}
          >
            <IconBookmark size={20} />
          </ActionIcon>
        </Tooltip>
        <Select
          aria-label="Add to list"
          placeholder="Choose list"
          value={null}
          onChange={(value) => {
            if (!value) return;
            if (value === "completed" && !window.confirm(completionQuestion)) return;
            add.mutate(value as "watching" | "watchlist" | "paused" | "dropped" | "completed");
          }}
          data={[
            { value: "watchlist", label: "Watchlist" },
            { value: "watching", label: "Watching" },
            { value: "paused", label: "Paused" },
            { value: "dropped", label: "Dropped" },
            ...(["Ended", "Canceled", "Cancelled"].includes(media.status ?? "")
              ? [{ value: "completed", label: "Completed" }]
              : []),
          ]}
          disabled={disabled || add.isPending}
          w={150}
        />
        {showWatchAction}
      </Group>
    );
  const statusOptions = [
    { value: "watchlist", label: "Watchlist" },
    { value: "watching", label: "Watching" },
    { value: "paused", label: "Paused" },
    { value: "dropped", label: "Dropped" },
    ...(["Ended", "Canceled", "Cancelled"].includes(media.status ?? "")
      ? [{ value: "completed", label: "Completed" }]
      : []),
  ];
  return (
    <Group className="detail-actions" mt="lg">
      <Select
        aria-label="Current list"
        allowDeselect={status !== "completed"}
        value={status}
        onChange={(value) => {
          if (value) {
            if (value === "completed" && status !== "completed") {
              if (!window.confirm(completionQuestion)) return;
              update.mutate({ status: value, rating: rating ?? null, confirm_all_episodes: true });
            } else update.mutate({ status: value, rating: rating ?? null });
          } else if (status === "watchlist") onRemoveWatchlist();
          else onRemoveCurrentList();
        }}
        data={status === "completed" ? [{ value: "completed", label: "Completed" }] : statusOptions}
        disabled={disabled || update.isPending}
        w={150}
      />
      {showWatchAction}
      <RatingStars
        value={rating}
        onChange={(value) => update.mutate({ status: status!, rating: value })}
        label="Media rating"
        disabled={disabled || update.isPending}
        size="md"
      />
      {status === "watching" && (
        <Button
          variant="default"
          onClick={onOpenNotifications}
          leftSection={
            notificationMode === "off" ? <IconBellOff size={20} /> : <IconBell size={20} />
          }
        >
          {notificationMode === "season"
            ? "Notifications · Every full season"
            : notificationMode === "episode"
              ? "Notifications · Every episode"
              : notificationMode === "off"
                ? "Notifications off"
                : "Choose notifications"}
        </Button>
      )}
    </Group>
  );
}
