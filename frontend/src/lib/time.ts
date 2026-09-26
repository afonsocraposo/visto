import TimeAgo from "javascript-time-ago";
import en from "javascript-time-ago/locale/en";

TimeAgo.addDefaultLocale(en);
const timeAgo = new TimeAgo("en-US");
const sevenDays = 7 * 24 * 60 * 60 * 1000;

export function formatActivityTime(value: string | Date, now = new Date()) {
  const date = value instanceof Date ? value : new Date(value);
  const elapsed = now.getTime() - date.getTime();
  if (elapsed >= 0 && elapsed < sevenDays) {
    return timeAgo.format(date, "round-minute", { now: now.getTime() });
  }
  return date.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
}
