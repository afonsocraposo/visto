import { ActionIcon, Badge, Group, Tooltip } from "@mantine/core";
import { IconBookmark, IconEye, IconEyeCheck } from "@tabler/icons-react";
import type { SearchMedia } from "../types";

type Props = {
  media: SearchMedia;
  saved?: boolean;
  savedLabel?: string;
  canMarkSavedWatched?: boolean;
  loadingAction?: "watch" | "watchlist" | "mark-watched";
  disabled?: boolean;
  onWatch: () => void;
  onWatchlist: () => void;
  onMarkSavedWatched?: () => void;
};

export function MediaQuickActions({
  media,
  saved = false,
  savedLabel = "In your library",
  canMarkSavedWatched = false,
  loadingAction,
  disabled = false,
  onWatch,
  onWatchlist,
  onMarkSavedWatched,
}: Props) {
  return (
    <Group
      className="media-quick-actions"
      gap="xs"
      onClick={(event) => event.stopPropagation()}
      onKeyDown={(event) => event.stopPropagation()}
    >
      {saved ? (
        <span key="saved" className="state-morph">
          <Badge color="teal" variant="light" leftSection={<IconEyeCheck size={14} />}>
            {savedLabel}
          </Badge>
          {canMarkSavedWatched && onMarkSavedWatched && (
            <Tooltip label="Mark watched" withArrow>
              <ActionIcon
                size={42}
                color="yellow"
                variant="light"
                aria-label={`Mark ${media.title} watched`}
                onClick={onMarkSavedWatched}
                loading={loadingAction === "mark-watched"}
                disabled={disabled}
              >
                <IconEye size={18} />
              </ActionIcon>
            </Tooltip>
          )}
        </span>
      ) : (
        <span key="unsaved" className="state-morph">
          <Tooltip label={media.type === "tv" ? "Add to watching" : "Mark watched"} withArrow>
            <ActionIcon
              size={42}
              color="yellow"
              variant="filled"
              aria-label={
                media.type === "tv"
                  ? `Add ${media.title} to watching`
                  : `Mark ${media.title} watched`
              }
              onClick={onWatch}
              loading={loadingAction === "watch"}
              disabled={disabled}
            >
              <IconEye size={18} />
            </ActionIcon>
          </Tooltip>
          <Tooltip label="Save for later" withArrow>
            <ActionIcon
              size={42}
              variant="default"
              aria-label={`Save ${media.title} for later`}
              onClick={onWatchlist}
              loading={loadingAction === "watchlist"}
              disabled={disabled}
            >
              <IconBookmark size={18} />
            </ActionIcon>
          </Tooltip>
        </span>
      )}
    </Group>
  );
}
