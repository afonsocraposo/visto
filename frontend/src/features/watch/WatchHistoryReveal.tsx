import { useLayoutEffect, useRef } from "react";
import { IconArrowUp } from "@tabler/icons-react";
import type { MediaDetailTarget } from "../../types";
import { RecentWatchRows, useRecentWatches } from "./RecentWatchHistory";

/**
 * The latest plays (oldest first), rendered above the "To watch" tabs and "Up next". The page
 * opens scrolled to just below this block, so scrolling up reveals the history in the normal page
 * scroll — no gesture, no open/closed state and no nested scroller.
 */
export function WatchHistoryReveal({
  onOpenDetail,
}: {
  onOpenDetail?: (target: MediaDetailTarget) => void;
}) {
  const history = useRecentWatches();
  const block = useRef<HTMLElement>(null);
  const positioned = useRef(false);
  const entries = history.data ?? [];

  // Start at "Up next" with the hint peeking, unless the user already scrolled or came back to
  // a restored scroll position.
  useLayoutEffect(() => {
    if (positioned.current || !entries.length || !block.current) return;
    positioned.current = true;
    if (window.scrollY > 0) return;
    const headerHeight =
      parseFloat(
        getComputedStyle(document.documentElement).getPropertyValue("--app-shell-header-height"),
      ) || 0;
    const hint = block.current.querySelector<HTMLElement>(".watch-history-hint");
    const top = (hint ?? block.current).getBoundingClientRect().top + window.scrollY;
    window.scrollTo({ top: Math.max(top - headerHeight - 8, 0), behavior: "instant" });
  }, [entries.length]);

  if (!entries.length) return null;
  return (
    <section ref={block} className="watch-history" aria-label="Recently watched">
      <div className="watch-list">
        <RecentWatchRows entries={entries} onOpenDetail={onOpenDetail} />
      </div>
      <p className="watch-history-hint" aria-hidden="true">
        <IconArrowUp size={14} /> Recent watches
      </p>
    </section>
  );
}
