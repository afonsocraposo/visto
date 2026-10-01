import type { AnchorHTMLAttributes, MouseEvent } from "react";

/** A route link that keeps the browser's native behavior for modified clicks. */
export function RouteLink({
  href,
  onOpen,
  ...props
}: Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "onClick"> & {
  href: string;
  onOpen?: () => void;
}) {
  return (
    <a
      href={href}
      onClick={(event: MouseEvent<HTMLAnchorElement>) => {
        if (
          !onOpen ||
          event.metaKey ||
          event.ctrlKey ||
          event.shiftKey ||
          event.altKey ||
          event.button !== 0
        )
          return;
        event.preventDefault();
        onOpen();
      }}
      {...props}
    />
  );
}
