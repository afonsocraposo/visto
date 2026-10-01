import dayjs from "dayjs";

/**
 * Release wording for an episode: "Today", "In N days" within the next week, otherwise the date
 * ("Mar 18", with the year when it differs). No air date means no release state at all.
 */
export function formatEpisodeAirDate(airDate: string | undefined | null, today = dayjs()) {
  if (!airDate) return null;
  const date = dayjs(airDate.slice(0, 10)).startOf("day");
  const days = date.diff(today.startOf("day"), "day");
  if (days === 0) return "Today";
  if (days === 1) return "Tomorrow";
  if (days >= 2 && days <= 7) return `In ${days} days`;
  return date.format(date.year() === today.year() ? "MMM D" : "MMM D, YYYY");
}

/** Whether an episode has not been released yet (unknown dates are not treated as future). */
export function isUnreleased(airDate: string | undefined | null, today = dayjs()) {
  return Boolean(airDate) && dayjs(airDate!.slice(0, 10)).isAfter(today, "day");
}
