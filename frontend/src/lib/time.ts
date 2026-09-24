const sevenDays = 7 * 24 * 60 * 60 * 1000;

export function formatActivityTime(value: string | Date, now = new Date()) {
  const date = value instanceof Date ? value : new Date(value);
  const elapsed = now.getTime() - date.getTime();
  if (elapsed >= 0 && elapsed < sevenDays) {
    const minutes = Math.floor(elapsed / 60_000);
    if (minutes < 1) return "Just now";
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    return days === 1 ? "Yesterday" : `${days}d ago`;
  }
  return date.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
}
