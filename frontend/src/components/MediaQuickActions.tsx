import { ActionIcon, Group, Tooltip } from "@mantine/core";
import { IconBookmark, IconEye, IconEyeCheck } from "@tabler/icons-react";
import type { SearchMedia } from "../types";

type Props = {
  media: SearchMedia;
  saved?: boolean;
  busy?: boolean;
  onWatch: () => void;
  onWatchlist: () => void;
};

export function MediaQuickActions({ media, saved = false, busy = false, onWatch, onWatchlist }: Props) {
  return <Group className="media-quick-actions" gap="xs" onClick={event => event.stopPropagation()} onKeyDown={event => event.stopPropagation()}>
    {saved ? <Tooltip label="Already in your library" withArrow><ActionIcon size="lg" variant="light" color="teal" aria-label={`${media.title} is in your library`}><IconEyeCheck size={18} /></ActionIcon></Tooltip> : <>
      <Tooltip label={media.type === "tv" ? "Add to watching" : "Mark watched"} withArrow><ActionIcon size="lg" color="yellow" variant="filled" aria-label={media.type === "tv" ? `Add ${media.title} to watching` : `Mark ${media.title} watched`} onClick={onWatch} loading={busy}><IconEye size={18} /></ActionIcon></Tooltip>
      <Tooltip label="Save for later" withArrow><ActionIcon size="lg" variant="default" aria-label={`Save ${media.title} for later`} onClick={onWatchlist} loading={busy}><IconBookmark size={18} /></ActionIcon></Tooltip>
    </>}
  </Group>;
}
