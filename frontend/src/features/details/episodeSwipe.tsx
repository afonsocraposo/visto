import type { ReactNode } from "react";
import { useDrag } from "@use-gesture/react";
import { swipeDirection, type SwipeDirection } from "./episodeNavigation";

/** Inputs, dialogs (portals bubble through React) and horizontal scrollers keep their own drags. */
function ownsHorizontalDrag(target: EventTarget | null) {
  for (let el = target instanceof Element ? target : null; el; el = el.parentElement) {
    if (el.matches("input, textarea, select, [role='slider'], [role='dialog']")) return true;
    if (el.scrollWidth > el.clientWidth && /auto|scroll/.test(getComputedStyle(el).overflowX))
      return true;
  }
  return false;
}

/** Swipe left → next, right → previous. Renders the element that receives the gesture. */
export function EpisodeSwipeArea({
  className,
  onSwipe,
  children,
}: {
  className: string;
  onSwipe?: (direction: SwipeDirection) => void;
  children: ReactNode;
}) {
  const bind = useDrag(
    ({ last, canceled, event, initial, xy }) => {
      if (!last || canceled || ownsHorizontalDrag(event.target)) return;
      const direction = swipeDirection(initial, xy, window.innerWidth);
      if (direction) onSwipe?.(direction);
    },
    { axis: "x", filterTaps: true, enabled: !!onSwipe },
  );
  return (
    <div className={className} {...bind()}>
      {children}
    </div>
  );
}
