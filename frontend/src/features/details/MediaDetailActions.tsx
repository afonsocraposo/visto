import type { ReactNode } from "react";
import { ActionIcon, Button, Group, Menu, Switch, Text } from "@mantine/core";
import {
  IconBell,
  IconBookmark,
  IconBookmarkFilled,
  IconCheck,
  IconChevronRight,
  IconDots,
  IconEye,
  IconEyeOff,
  IconHistory,
  IconEdit,
  IconPlus,
  IconRefresh,
  IconTrash,
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

export type ListStatus = "watching" | "watchlist" | "paused" | "dropped" | "completed";

export const statusLabels: Record<ListStatus, string> = {
  watching: "Watching",
  watchlist: "Watchlist",
  paused: "Paused",
  dropped: "Dropped",
  completed: "Completed",
};

function listOptions(media: SearchMedia): ListStatus[] {
  if (media.type === "movie") return ["watchlist"];
  const ended = ["Ended", "Canceled", "Cancelled"].includes(media.status ?? "");
  return ["watching", "watchlist", "paused", "dropped", ...(ended ? ["completed" as const] : [])];
}

/** Picks the list a title belongs to. Used by the hero status button and the tracking row. */
export function StatusMenu({
  media,
  status,
  onChange,
  onRemove,
  disabled,
  children,
}: {
  media: SearchMedia;
  status?: ListStatus;
  onChange: (status: ListStatus) => void;
  onRemove?: () => void;
  disabled?: boolean;
  children: ReactNode;
}) {
  const options =
    status === "completed" && media.type === "tv" ? ["completed" as const] : listOptions(media);
  return (
    <Menu withinPortal position="bottom-start" disabled={disabled}>
      <Menu.Target>{children}</Menu.Target>
      <Menu.Dropdown>
        <Menu.Label>Move to list</Menu.Label>
        {options.map((option) => (
          <Menu.Item
            key={option}
            rightSection={status === option ? <IconCheck size={15} /> : undefined}
            onClick={() => {
              if (option === status) return;
              if (
                option === "completed" &&
                media.type === "tv" &&
                !window.confirm(completionQuestion)
              )
                return;
              onChange(option);
            }}
          >
            {statusLabels[option]}
            {status === option && <span className="visually-hidden"> (current)</span>}
          </Menu.Item>
        ))}
        {status && onRemove && status !== "completed" && (
          <>
            <Menu.Divider />
            <Menu.Item color="red" leftSection={<IconTrash size={15} />} onClick={onRemove}>
              Remove from {statusLabels[status]}
            </Menu.Item>
          </>
        )}
      </Menu.Dropdown>
    </Menu>
  );
}

type MediaActionsProps = {
  media: SearchMedia;
  isSaved: boolean;
  status?: ListStatus;
  add: ActionMutation<ListStatus>;
  watched: boolean;
  onWatch: () => void;
  onUnwatch: () => void;
  onRemoveWatchlist: () => void;
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

/**
 * The hero's primary actions: at most two labelled buttons and an overflow menu, so touch users
 * never depend on icon-only controls and tooltips.
 */
export function MediaActions({
  media,
  isSaved,
  status,
  add,
  watched,
  onWatch,
  onUnwatch,
  onRemoveWatchlist,
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
  const overflow = (items: ReactNode, label: string) => (
    <Menu withinPortal position="bottom-end">
      <Menu.Target>
        <ActionIcon
          className="detail-overflow-action"
          size={42}
          variant="default"
          aria-label={label}
          disabled={disabled}
        >
          <IconDots size={18} />
        </ActionIcon>
      </Menu.Target>
      <Menu.Dropdown>{items}</Menu.Dropdown>
    </Menu>
  );

  if (media.type === "movie") {
    const inWatchlist = isSaved && status === "watchlist";
    return (
      <Group className="detail-actions" mt="lg" gap="xs">
        <Button
          className="detail-primary-action"
          color={watched ? "teal" : "yellow"}
          variant={watched ? "light" : "filled"}
          leftSection={watched ? <IconCheck size={18} /> : <IconEye size={18} />}
          aria-label={`Mark ${media.title} ${watched ? "unwatched" : "watched"}`}
          onClick={watched ? onUnwatch : onWatch}
          loading={watched ? removePending : watchPending}
          disabled={disabled}
        >
          {watched ? "Watched" : "Mark watched"}
        </Button>
        {!watched && (
          <ToggleButton
            active={inWatchlist}
            activeLabel="In Watchlist"
            label="Watchlist"
            activeIcon={<IconBookmarkFilled size={17} />}
            icon={<IconBookmark size={17} />}
            aria-label={
              inWatchlist ? `Remove ${media.title} from Watchlist` : `Save ${media.title} for later`
            }
            onClick={inWatchlist ? onRemoveWatchlist : () => add.mutate("watchlist")}
            loading={watchlistPending || (add.isPending && add.variables === "watchlist")}
            disabled={disabled}
          />
        )}
        {watched &&
          overflow(
            <>
              <Menu.Item leftSection={<IconRefresh size={16} />} onClick={onWatch}>
                Mark rewatched
              </Menu.Item>
              <Menu.Item leftSection={<IconHistory size={16} />} onClick={onViewWatchHistory}>
                View watch history
              </Menu.Item>
              <Menu.Item leftSection={<IconEdit size={16} />} onClick={onChangeWatchDate}>
                Change watch date
              </Menu.Item>
              <Menu.Divider />
              <Menu.Item leftSection={<IconEyeOff size={16} />} onClick={onUnwatch}>
                Mark unwatched
              </Menu.Item>
            </>,
            `More actions for ${media.title}`,
          )}
      </Group>
    );
  }

  const bulkWatch = showBulkAction === "watch" && (
    <Button
      className="show-bulk-action"
      variant={isSaved ? "filled" : "default"}
      color={isSaved ? "yellow" : undefined}
      leftSection={<IconEye size={18} />}
      aria-label={`Mark ${media.title} watched`}
      onClick={onShowBulkAction}
      loading={showBulkPending}
      disabled={disabled || add.isPending}
    >
      Mark watched
    </Button>
  );
  const bulkUnwatchItem = showBulkAction === "unwatch" && (
    <Menu.Item leftSection={<IconEyeOff size={16} />} onClick={onShowBulkAction}>
      Mark {media.title} unwatched
    </Menu.Item>
  );

  if (!isSaved)
    return (
      <Group className="detail-actions" mt="lg" gap="xs">
        <Button
          className="detail-primary-action"
          leftSection={<IconPlus size={18} />}
          loading={add.isPending && add.variables === "watching"}
          disabled={disabled || add.isPending}
          aria-label={`Add ${media.title} to Watching`}
          onClick={() => add.mutate("watching")}
        >
          Watching
        </Button>
        <Button
          variant="default"
          leftSection={<IconBookmark size={17} />}
          loading={add.isPending && add.variables === "watchlist"}
          disabled={disabled || add.isPending}
          aria-label={`Save ${media.title} for later`}
          onClick={() => add.mutate("watchlist")}
        >
          Watchlist
        </Button>
        {overflow(
          <>
            <Menu.Label>Add to list</Menu.Label>
            {listOptions(media)
              .filter((option) => option !== "watching" && option !== "watchlist")
              .map((option) => (
                <Menu.Item
                  key={option}
                  onClick={() => {
                    if (option === "completed" && !window.confirm(completionQuestion)) return;
                    add.mutate(option);
                  }}
                >
                  {statusLabels[option]}
                </Menu.Item>
              ))}
            {showBulkAction === "watch" && (
              <>
                <Menu.Divider />
                <Menu.Item leftSection={<IconEye size={16} />} onClick={onShowBulkAction}>
                  Mark seasons watched…
                </Menu.Item>
              </>
            )}
            {bulkUnwatchItem}
          </>,
          `More actions for ${media.title}`,
        )}
        {/* Bulk marking stays reachable as a labelled button on wide screens. */}
        <span className="detail-actions-wide">{bulkWatch}</span>
      </Group>
    );

  // The list itself is changed from "Your tracking" right below; the hero only keeps actions.
  return (
    <Group className="detail-actions" mt="lg" gap="xs">
      {bulkWatch}
      {bulkUnwatchItem && overflow(bulkUnwatchItem, `More actions for ${media.title}`)}
    </Group>
  );
}

function ToggleButton({
  active,
  label,
  activeLabel,
  icon,
  activeIcon,
  ...props
}: {
  active: boolean;
  label: string;
  activeLabel: string;
  icon: ReactNode;
  activeIcon: ReactNode;
  "aria-label": string;
  onClick: () => void;
  loading: boolean;
  disabled: boolean;
}) {
  return (
    <Button
      className="state-toggle"
      data-active={active || undefined}
      variant={active ? "light" : "default"}
      aria-pressed={active}
      leftSection={
        <span key={String(active)} className="state-toggle-icon">
          {active ? activeIcon : icon}
        </span>
      }
      {...props}
    >
      {active ? activeLabel : label}
    </Button>
  );
}

const alertModeLabels = {
  episode: "Every episode",
  season: "Every full season",
  off: "Off",
  "": "Choose",
} as const;

/** Everything personal about a saved title in one predictable place below the hero. */
export function TrackingSection({
  media,
  status,
  rating,
  onRate,
  ratingPending,
  onChangeStatus,
  onRemove,
  statusPending,
  alerts,
}: {
  media: SearchMedia;
  status?: ListStatus;
  rating?: number | null;
  onRate: (rating: number | null) => void;
  ratingPending: boolean;
  onChangeStatus: (status: ListStatus) => void;
  onRemove: () => void;
  statusPending: boolean;
  alerts:
    | { kind: "show"; mode: "episode" | "season" | "off" | ""; onOpen: () => void }
    | { kind: "movie"; enabled: boolean; pending: boolean; onToggle: (enabled: boolean) => void }
    | null;
}) {
  return (
    <section className="tracking-section" aria-labelledby="tracking-heading">
      <h2 id="tracking-heading" className="tracking-heading">
        Your tracking
      </h2>
      <div className="tracking-rows">
        <StatusMenu
          media={media}
          status={status}
          onChange={onChangeStatus}
          onRemove={onRemove}
          disabled={statusPending}
        >
          <button
            type="button"
            className="tracking-row"
            aria-label={`Status: ${status ? statusLabels[status] : "None"}. Change list`}
          >
            <span className="tracking-row-label">Status</span>
            <span className="tracking-row-value">{status ? statusLabels[status] : "None"}</span>
            <IconChevronRight className="tracking-row-chevron" size={18} aria-hidden="true" />
          </button>
        </StatusMenu>
        <div className="tracking-row is-static">
          <span className="tracking-row-label" id="tracking-rating-label">
            Your rating
          </span>
          <RatingStars
            value={rating}
            onChange={onRate}
            label="Media rating"
            disabled={ratingPending}
            size="md"
          />
        </div>
        {alerts?.kind === "show" && (
          <button
            type="button"
            className="tracking-row"
            aria-label={`Alerts: ${alertModeLabels[alerts.mode]}. Change alerts`}
            onClick={alerts.onOpen}
          >
            <span className="tracking-row-label">
              <IconBell size={16} aria-hidden="true" /> Alerts
            </span>
            <span className="tracking-row-value">{alertModeLabels[alerts.mode]}</span>
            <IconChevronRight className="tracking-row-chevron" size={18} aria-hidden="true" />
          </button>
        )}
        {alerts?.kind === "movie" && (
          <label className="tracking-row">
            <span className="tracking-row-label">
              <IconBell size={16} aria-hidden="true" /> Release alert
            </span>
            <Switch
              checked={alerts.enabled}
              disabled={alerts.pending}
              aria-label={movieReleaseAlertLabel(alerts.enabled)}
              onChange={(event) => alerts.onToggle(event.currentTarget.checked)}
            />
          </label>
        )}
      </div>
      {!status && (
        <Text size="xs" c="dimmed" mt={6}>
          Add this title to a list to keep your rating and alerts.
        </Text>
      )}
    </section>
  );
}

export { showMovieReleaseAlert };
