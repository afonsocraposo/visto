import type { AnchorHTMLAttributes, MouseEvent } from "react";
import { detailPath } from "../features/navigation/detailPath";
import type { MediaDetailTarget } from "../types";

/**
 * A real link to a media/episode page: it opens in a new tab with modifier keys or the context
 * menu, while a plain click goes through the app's navigation (keeping the return path).
 */
export function DetailLink({
  to,
  onOpen,
  ...props
}: Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "href" | "onClick" | "target"> & {
  to: MediaDetailTarget;
  onOpen?: (target: MediaDetailTarget) => void;
}) {
  return (
    <a
      href={detailPath(to)}
      onClick={(event: MouseEvent<HTMLAnchorElement>) => {
        if (!onOpen) return;
        if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0)
          return;
        event.preventDefault();
        onOpen(to);
      }}
      {...props}
    />
  );
}
