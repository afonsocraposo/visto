import { useLayoutEffect, useRef, useState } from "react";
import { useDrag } from "@use-gesture/react";
import { Alert, Loader } from "@mantine/core";
import { IconChevronDown, IconChevronUp } from "@tabler/icons-react";
import type { MediaDetailTarget } from "../../types";
import { RecentWatchRows, useRecentWatches } from "./RecentWatchHistory";
import { WatchNow } from "./Watch";

const PULL_OPEN = 70;
const PULL_MAX = 110;
/** How much of the history is scrolled into view when it opens. */
const PEEK = 96;

/**
 * Pulling down at the top of "Episodes to watch" extends the same list upwards with the
 * latest plays (oldest first), keeping the rows you were looking at where they are.
 */
export function WatchHistoryReveal({
  onOpenDetail,
}: {
  onOpenDetail?: (target: MediaDetailTarget) => void;
}) {
  const history = useRecentWatches();
  const [wanted, setWanted] = useState(false);
  const [pull, setPull] = useState(0);
  const [settling, setSettling] = useState<{ offset: number; animate: boolean } | null>(null);
  const block = useRef<HTMLDivElement>(null);
  const releasedAt = useRef(0);
  const revealed = wanted && history.isSuccess;

  const bind = useDrag(
    ({ last, cancel, movement: [, my] }) => {
      if (window.scrollY > 0) {
        cancel();
        return setPull(0);
      }
      const distance = Math.min(Math.max(my, 0), PULL_MAX);
      if (last) {
        releasedAt.current = distance / 2;
        if (my > PULL_OPEN) setWanted(true);
        return setPull(0);
      }
      setPull(distance);
    },
    { axis: "y", enabled: !wanted, filterTaps: true, pointer: { touch: true } },
  );
  // Touch mode adds a click-capture handler that swallows mouse clicks (hybrid devices).
  const { onClickCapture: _swallowsMouseClicks, ...dragHandlers } = bind();

  // The history is inserted above the rows being read: scroll by its height (minus a peek)
  // before paint so those rows stay put, then ease the peek in.
  useLayoutEffect(() => {
    if (!revealed || !block.current) return;
    const released = releasedAt.current;
    releasedAt.current = 0;
    const gap = parseFloat(getComputedStyle(block.current.parentElement!).rowGap) || 0;
    window.scrollTo(0, Math.max(block.current.offsetHeight + gap - PEEK, 0));
    setSettling({ offset: released - PEEK, animate: false });
    const frame = requestAnimationFrame(() => setSettling({ offset: 0, animate: true }));
    return () => cancelAnimationFrame(frame);
  }, [revealed]);

  const offset = pull > 0 ? pull / 2 : (settling?.offset ?? 0);
  const animate = pull === 0 && (settling?.animate ?? true);

  return (
    <div
      className="watch-history-reveal"
      style={{
        transform: offset ? `translateY(${offset}px)` : undefined,
        transition: animate ? "transform 220ms ease-out" : "none",
      }}
      {...dragHandlers}
    >
      <div
        className="watch-history-hint"
        style={{ height: offset, opacity: Math.min(pull / PULL_OPEN, 1) }}
      >
        <IconChevronDown size={14} /> <span>Recent watches</span>
      </div>
      <button
        type="button"
        className="watch-history-reveal-header"
        aria-expanded={revealed}
        onClick={() => {
          releasedAt.current = 0;
          setWanted((value) => !value);
        }}
      >
        {revealed ? <IconChevronUp size={14} /> : <IconChevronDown size={14} />}
        <span>{revealed ? "Hide recent watches" : "Recent watches"}</span>
      </button>
      {wanted && history.isError && (
        <Alert color="red" variant="light" mb="sm">
          Watch history is temporarily unavailable.
        </Alert>
      )}
      {wanted && history.isPending && <Loader size="xs" mb="sm" />}
      <WatchNow
        onOpenDetail={onOpenDetail}
        before={
          revealed && (
            <div ref={block} className="watch-history-block" aria-label="Recently watched">
              <RecentWatchRows entries={history.data} onOpenDetail={onOpenDetail} />
              <div className="watch-history-divider" role="separator">
                <span>Up next</span>
              </div>
            </div>
          )
        }
      />
    </div>
  );
}
