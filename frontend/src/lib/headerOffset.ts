/** Height of the sticky top app bar (0 on mobile, where navigation is at the bottom). */
export function headerOffset(): number {
  const bar = document.querySelector<HTMLElement>(".visto-topbar");
  return bar && getComputedStyle(bar).display !== "none" ? bar.getBoundingClientRect().height : 0;
}
