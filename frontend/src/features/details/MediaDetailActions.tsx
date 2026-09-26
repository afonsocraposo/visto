import { ActionIcon, Group, Menu, Select, Switch, Text, Tooltip } from "@mantine/core";
import {
  IconBookmark,
  IconCheck,
  IconChevronDown,
  IconEye,
  IconHistory,
  IconEdit,
  IconPlus,
  IconRefresh,
} from "@tabler/icons-react";
import { RatingStars } from "../../components/RatingStars";
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
    <Group className="detail-actions detail-icon-actions" mt="lg" gap="xs">
      <Tooltip label={entry.watched ? "Mark unwatched" : "Mark watched"} withArrow>
        <ActionIcon
          size="lg"
          color="yellow"
          variant="filled"
          aria-label={`Mark episode ${entry.episode.episode_number} ${entry.watched ? "unwatched" : "watched"}`}
          onClick={entry.watched ? onUnwatch : onWatch}
          loading={pending}
        >
          {entry.watched ? <IconCheck size={18} /> : <IconEye size={18} />}
        </ActionIcon>
      </Tooltip>
      {entry.watched && (
        <Menu withinPortal position="bottom-start">
          <Menu.Target>
            <ActionIcon
              size="lg"
              variant="default"
              aria-label="More episode actions"
              disabled={pending}
            >
              <IconChevronDown size={18} />
            </ActionIcon>
          </Menu.Target>
          <Menu.Dropdown>
            <Menu.Item leftSection={<IconRefresh size={16} />} onClick={onRewatch}>
              Mark rewatched
            </Menu.Item>
          </Menu.Dropdown>
        </Menu>
      )}
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
  onRemoveWatchlist: () => void;
  onRemoveCurrentList: () => void;
  onViewWatchHistory: () => void;
  onChangeWatchDate: () => void;
  showBulkAction?: "watch" | "unwatch" | null;
  onShowBulkAction: () => void;
  showBulkPending: boolean;
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
  onRemoveWatchlist,
  onRemoveCurrentList,
  onViewWatchHistory,
  onChangeWatchDate,
  showBulkAction,
  onShowBulkAction,
  showBulkPending,
  pending,
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
            loading={pending}
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
                disabled={pending}
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
              loading={pending || add.isPending}
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
            disabled={pending}
            size="md"
          />
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
        disabled={pending || add.isPending}
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
            loading={add.isPending}
            disabled={pending}
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
            loading={add.isPending}
            disabled={pending}
            aria-label={`Save ${media.title} for later`}
            onClick={() => add.mutate("watchlist")}
          >
            <IconBookmark size={20} />
          </ActionIcon>
        </Tooltip>
        {showWatchAction}
      </Group>
    );
  const statusOptions = [
    { value: "watchlist", label: "Watchlist" },
    { value: "watching", label: "Watching" },
    { value: "paused", label: "Paused" },
    { value: "dropped", label: "Dropped" },
  ];
  return (
    <Group className="detail-actions" mt="lg">
      <Select
        aria-label="Current list"
        allowDeselect
        value={status}
        onChange={(value) => {
          if (value) update.mutate({ status: value, rating: rating ?? null });
          else if (status === "watchlist") onRemoveWatchlist();
          else onRemoveCurrentList();
        }}
        data={statusOptions}
        disabled={pending || update.isPending}
        w={150}
      />
      {showWatchAction}
      <RatingStars
        value={rating}
        onChange={(value) => update.mutate({ status: status!, rating: value })}
        label="Media rating"
        disabled={pending}
        size="md"
      />
      <Switch
        aria-label="New episode notifications for this show"
        label="Episode alerts"
        checked={notificationsEnabled}
        disabled={updateNotifications.isPending}
        onChange={(event) => updateNotifications.mutate(event.currentTarget.checked)}
      />
    </Group>
  );
}
