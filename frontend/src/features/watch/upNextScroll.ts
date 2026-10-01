/**
 * Choosing "Watching" always lands on "Up next" (the recent history sits just above it).
 * The request is timestamped rather than consumed, so it survives the page not being mounted
 * yet and effect re-runs, and it is also broadcast so tapping the tab while already on
 * Watching returns to Up next.
 */
const EVENT = "visto:scroll-to-up-next";
const FRESH_MS = 3000;
let requestedAt = -Infinity;

export function requestUpNextScroll() {
  requestedAt = performance.now();
  window.dispatchEvent(new Event(EVENT));
}

/** Whether Watching was chosen a moment ago (so the page that opens should start at Up next). */
export function upNextRequestedRecently(): boolean {
  return performance.now() - requestedAt < FRESH_MS;
}

export function onUpNextRequest(listener: () => void) {
  window.addEventListener(EVENT, listener);
  return () => window.removeEventListener(EVENT, listener);
}
