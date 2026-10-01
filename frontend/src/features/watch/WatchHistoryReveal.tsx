import { useEffect, useLayoutEffect, useRef } from "react";
import { IconArrowUp } from "@tabler/icons-react";
import type { MediaDetailTarget } from "../../types";
import { headerOffset } from "../../lib/headerOffset";
import { RecentWatchRows, useRecentWatches } from "./RecentWatchHistory";
import { onUpNextRequest, upNextRequestedRecently } from "./upNextScroll";

/**
 * The latest plays (oldest first), rendered above the "To watch" tabs and "Up next". The page
 * starts scrolled to just below this block, so scrolling up reveals the history in the normal
 * page scroll — no gesture, no open/closed state and no nested scroller.
 */
export function WatchHistoryReveal({
  onOpenDetail,
}: {
  onOpenDetail?: (target: MediaDetailTarget) => void;
}) {
  const history = useRecentWatches();
  const block = useRef<HTMLElement>(null);
  const stopSettling = useRef<() => void>(() => {});
  const entries = history.data ?? [];
  const ready = entries.length > 0;

  const scrollToUpNext = () => {
    const hint = block.current?.querySelector<HTMLElement>(".watch-history-hint");
    if (!hint) return;
    const top = hint.getBoundingClientRect().top + window.scrollY - headerOffset() - 8;
    const wanted = Math.max(top, 0);
    // Only move when needed, so the scroll listener never feeds on its own scrolling.
    if (Math.abs(window.scrollY - wanted) > 1)
      window.scrollTo({ top: wanted, behavior: "instant" });
  };

  /**
   * Positions now and keeps the position while the page below is still loading or the router
   * resets scroll, until the user takes over (touch, wheel, keys) or a short while has passed.
   */
  const settleOnUpNext = () => {
    stopSettling.current();
    scrollToUpNext();
    const target = block.current?.parentElement;
    if (!target) return;
    const reapply = () => scrollToUpNext();
    const observer = new ResizeObserver(reapply);
    observer.observe(target);
    // The router resets the scroll right after a navigation; undo that, but only briefly.
    window.addEventListener("scroll", reapply, { passive: true });
    const stopScrollGuard = window.setTimeout(
      () => window.removeEventListener("scroll", reapply),
      600,
    );
    const stop = () => {
      observer.disconnect();
      window.clearTimeout(stopScrollGuard);
      window.removeEventListener("scroll", reapply);
      window.clearTimeout(timer);
      for (const type of ["wheel", "touchstart", "keydown", "pointerdown"])
        window.removeEventListener(type, stop);
      stopSettling.current = () => {};
    };
    const timer = window.setTimeout(stop, 2500);
    for (const type of ["wheel", "touchstart", "keydown", "pointerdown"])
      window.addEventListener(type, stop, { passive: true });
    stopSettling.current = stop;
  };

  // First time the history is available: start at Up next when Watching was chosen (or opened
  // directly at the top). Coming back through history keeps the restored scroll position. The
  // decision is made once per visit, so effect re-runs (StrictMode) don't change it.
  const wantUpNext = useRef<boolean | null>(null);
  useLayoutEffect(() => {
    if (!ready) return;
    if (wantUpNext.current === null)
      wantUpNext.current = upNextRequestedRecently() || window.scrollY === 0;
    if (wantUpNext.current) settleOnUpNext();
    return () => stopSettling.current();
  }, [ready]);

  // Tapping Watching while already here goes back to Up next.
  useEffect(() => {
    return onUpNextRequest(() => {
      if (!block.current) return;
      wantUpNext.current = true;
      settleOnUpNext();
    });
  }, []);

  if (!ready) return null;
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
