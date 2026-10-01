import dayjs from "dayjs";

/** The local calendar day a timestamp falls on, used to group activity. */
export function localDayKey(value: string | Date): string {
  return dayjs(value).format("YYYY-MM-DD");
}

/** "Today", "Yesterday", or a calendar date ("28 September", with the year when it differs). */
export function relativeDayLabel(dayKey: string, now: Date = new Date()): string {
  const day = dayjs(dayKey).startOf("day");
  const today = dayjs(now).startOf("day");
  const days = today.diff(day, "day");
  if (days === 0) return "Today";
  if (days === 1) return "Yesterday";
  return day.format(day.year() === today.year() ? "D MMMM" : "D MMMM YYYY");
}

/** Groups items by local day, keeping their order (newest first stays newest first). */
export function groupByLocalDay<T>(items: T[], dateOf: (item: T) => string | Date) {
  const groups: Array<{ day: string; items: T[] }> = [];
  for (const item of items) {
    const day = localDayKey(dateOf(item));
    const last = groups.at(-1);
    if (last?.day === day) last.items.push(item);
    else groups.push({ day, items: [item] });
  }
  return groups;
}
