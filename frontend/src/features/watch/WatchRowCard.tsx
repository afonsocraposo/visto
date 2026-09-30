import type { ReactNode } from "react";
import { Image, Paper } from "@mantine/core";

/** The card shared by "Episodes to watch" rows and the recent watch history rows. */
export function WatchRowCard({
  className = "",
  title,
  art,
  onOpen,
  onAnimationEnd,
  children,
  trailing,
}: {
  className?: string;
  title: string;
  art?: string | null;
  onOpen?: () => void;
  onAnimationEnd?: (event: React.AnimationEvent<HTMLDivElement>) => void;
  children: ReactNode;
  trailing?: ReactNode;
}) {
  return (
    <Paper
      className={`watch-row ${className}`.trim()}
      withBorder
      p={0}
      role={onOpen ? "button" : undefined}
      tabIndex={onOpen ? 0 : undefined}
      onAnimationEnd={onAnimationEnd}
      onClick={onOpen}
      onKeyDown={(event) => {
        if (
          onOpen &&
          (event.key === "Enter" || event.key === " ") &&
          event.target === event.currentTarget
        ) {
          event.preventDefault();
          onOpen();
        }
      }}
    >
      <div className="watch-row-art">
        {art ? (
          <Image src={art} alt="" />
        ) : (
          <div className="artwork-fallback">{title.slice(0, 1)}</div>
        )}
      </div>
      <div className="watch-row-content">{children}</div>
      {trailing}
    </Paper>
  );
}
