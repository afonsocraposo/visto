import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
} from "react";
import { useDrag } from "@use-gesture/react";
import { dragFollow, swipeDirection, type SwipeDirection } from "./episodeNavigation";

/** Exit/enter distance in px and duration in ms of the directional episode slide. */
const SLIDE_DISTANCE = 20;
const SLIDE_MS = 200;

type Motion = { shift: number; opacity: number; mode: "none" | "snap" | "slide" };
const REST: Motion = { shift: 0, opacity: 1, mode: "snap" };

const EpisodeNavigateContext = createContext<(direction: SwipeDirection) => void>(() => {});

/** Opens the adjacent episode through the same slide used by swipes. */
export function useEpisodeNavigate() {
  return useContext(EpisodeNavigateContext);
}

function prefersReducedMotion() {
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

/** Inputs, dialogs (portals bubble through React) and horizontal scrollers keep their own drags. */
function ownsHorizontalDrag(target: EventTarget | null) {
  for (let el = target instanceof Element ? target : null; el; el = el.parentElement) {
    if (el.matches("input, textarea, select, [role='slider'], [role='dialog']")) return true;
    if (el.scrollWidth > el.clientWidth && /auto|scroll/.test(getComputedStyle(el).overflowX))
      return true;
  }
  return false;
}

/**
 * Wraps an episode page. While dragging horizontally the content follows the finger a little;
 * releasing past the threshold (or using previous/next) slides the current episode out, opens
 * the adjacent one and slides it in from the other side. Short drags snap back.
 */
export function EpisodeTransition({
  className,
  episodeKey,
  available,
  onNavigate,
  children,
}: {
  className: string;
  /** Changes when a different episode is shown; triggers the enter half of the slide. */
  episodeKey: string | null;
  available: Record<SwipeDirection, boolean>;
  onNavigate: (direction: SwipeDirection) => void;
  children: ReactNode;
}) {
  const [motion, setMotion] = useState<Motion>(REST);
  const pending = useRef<SwipeDirection | null>(null);
  const exitTimer = useRef<number | undefined>(undefined);
  const enabled = episodeKey !== null;

  const navigate = useCallback(
    (direction: SwipeDirection) => {
      if (!available[direction] || pending.current) return;
      if (prefersReducedMotion()) return onNavigate(direction);
      pending.current = direction;
      setMotion({
        shift: direction === "next" ? -SLIDE_DISTANCE : SLIDE_DISTANCE,
        opacity: 0,
        mode: "slide",
      });
      exitTimer.current = window.setTimeout(() => onNavigate(direction), SLIDE_MS);
    },
    [available, onNavigate],
  );

  // The new episode starts on the side it comes from (without a transition), then settles.
  useLayoutEffect(() => {
    const direction = pending.current;
    if (!direction) return;
    pending.current = null;
    setMotion({
      shift: direction === "next" ? SLIDE_DISTANCE : -SLIDE_DISTANCE,
      opacity: 0,
      mode: "none",
    });
    let second = 0;
    const first = requestAnimationFrame(() => {
      second = requestAnimationFrame(() => setMotion({ shift: 0, opacity: 1, mode: "slide" }));
    });
    return () => {
      cancelAnimationFrame(first);
      cancelAnimationFrame(second);
    };
  }, [episodeKey]);

  useEffect(() => () => window.clearTimeout(exitTimer.current), []);

  const bind = useDrag(
    ({ first, last, canceled, cancel, event, initial, xy, movement: [mx] }) => {
      if (pending.current) return;
      if (first && ownsHorizontalDrag(event.target)) return cancel();
      if (canceled) return setMotion(REST);
      if (!last) {
        if (prefersReducedMotion()) return;
        const direction: SwipeDirection = mx < 0 ? "next" : "previous";
        return setMotion({ ...dragFollow(mx, available[direction]), mode: "none" });
      }
      const direction = swipeDirection(initial, xy, window.innerWidth);
      if (direction && available[direction]) navigate(direction);
      else setMotion(REST);
    },
    { axis: "x", filterTaps: true, enabled },
  );

  return (
    <EpisodeNavigateContext.Provider value={navigate}>
      <div
        className={`${className}${enabled ? " episode-transition" : ""}`}
        data-motion={motion.mode}
        style={
          enabled
            ? ({
                "--episode-shift": `${motion.shift}px`,
                "--episode-opacity": motion.opacity,
              } as CSSProperties)
            : undefined
        }
        {...(enabled ? bind() : {})}
      >
        {children}
      </div>
    </EpisodeNavigateContext.Provider>
  );
}
