import { useState, type ReactNode } from "react";
import { useDrag } from "@use-gesture/react";
import { IconChevronDown, IconChevronUp } from "@tabler/icons-react";
import type { MediaDetailTarget } from "../../types";
import { RecentWatchHistory } from "./RecentWatchHistory";

const PULL_OPEN = 70;
const PULL_MAX = 110;

export function WatchHistoryReveal({
  onOpenDetail,
  children,
}: {
  onOpenDetail?: (target: MediaDetailTarget) => void;
  children: ReactNode;
}) {
  const [revealed, setRevealed] = useState(false);
  const [pull, setPull] = useState(0);
  const bind = useDrag(
    ({ last, cancel, movement: [, my] }) => {
      if (window.scrollY > 0) {
        cancel();
        return setPull(0);
      }
      if (last && my > PULL_OPEN) setRevealed(true);
      setPull(last ? 0 : Math.min(Math.max(my, 0), PULL_MAX));
    },
    { axis: "y", enabled: !revealed, filterTaps: true, pointer: { touch: true } },
  );
  // Touch mode adds a click-capture handler that swallows mouse clicks (hybrid devices).
  const { onClickCapture: _swallowsMouseClicks, ...dragHandlers } = bind();

  return (
    <div className="watch-history-reveal" {...dragHandlers}>
      <button
        type="button"
        className="watch-history-reveal-header"
        aria-expanded={revealed}
        style={pull > 0 ? { height: pull, opacity: Math.min(pull / PULL_OPEN, 1) } : undefined}
        onClick={() => setRevealed((value) => !value)}
      >
        {revealed ? <IconChevronUp size={14} /> : <IconChevronDown size={14} />}
        <span>{revealed ? "Hide recent watches" : "Recent watches"}</span>
      </button>
      {revealed && (
        <section className="watch-history-section" aria-label="Recently watched">
          <RecentWatchHistory onOpenDetail={onOpenDetail} />
        </section>
      )}
      {children}
    </div>
  );
}
