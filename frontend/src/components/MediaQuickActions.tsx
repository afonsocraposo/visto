import { ActionIcon, Badge, Group, Tooltip } from "@mantine/core";
import { IconBookmark, IconEye, IconEyeCheck } from "@tabler/icons-react";
import type { SearchMedia } from "../types";

type Props = {
  media: SearchMedia;
  saved?: boolean;
  savedLabel?: string;
  canMarkSavedWatched?: boolean;
  busy?: boolean;
  onWatch: () => void;
  onWatchlist: () => void;
  onMarkSavedWatched?: () => void;
};

export function MediaQuickActions({
  media,
  saved = false,
  savedLabel = "In your library",
  canMarkSavedWatched = false,
  busy = false,
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
        <>
          <Badge color="teal" variant="light" leftSection={<IconEyeCheck size={14} />}>
            {savedLabel}
          </Badge>
          {canMarkSavedWatched && onMarkSavedWatched && (
            <Tooltip label="Mark watched" withArrow>
              <ActionIcon
                size="lg"
                color="yellow"
                variant="light"
                aria-label={`Mark ${media.title} watched`}
                onClick={onMarkSavedWatched}
                loading={busy}
              >
                <IconEye size={18} />
              </ActionIcon>
            </Tooltip>
          )}
        </>
      ) : (
        <>
          <Tooltip label={media.type === "tv" ? "Add to watching" : "Mark watched"} withArrow>
            <ActionIcon
              size="lg"
              color="yellow"
              variant="filled"
              aria-label={
                media.type === "tv"
                  ? `Add ${media.title} to watching`
                  : `Mark ${media.title} watched`
              }
              onClick={onWatch}
              loading={busy}
            >
              <IconEye size={18} />
            </ActionIcon>
          </Tooltip>
          <Tooltip label="Save for later" withArrow>
            <ActionIcon
              size="lg"
              variant="default"
              aria-label={`Save ${media.title} for later`}
              onClick={onWatchlist}
              loading={busy}
            >
              <IconBookmark size={18} />
            </ActionIcon>
          </Tooltip>
        </>
      )}
    </Group>
  );
}
