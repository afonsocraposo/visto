import type { CalendarEntry } from "../../types";

export type CalendarGroup = { date: string; entries: CalendarEntry[] };

export function dateInTimezone(timeZone: string, instant = new Date()): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(instant);
  const part = (type: string) => parts.find(value => value.type === type)?.value ?? "";
  return `${part("year")}-${part("month")}-${part("day")}`;
}

export function monthInTimezone(timeZone: string, instant = new Date()): string {
  return dateInTimezone(timeZone, instant).slice(0, 7);
}

export function shiftCalendarMonth(month: string, amount: number): string {
  const [year, index] = month.split("-").map(Number);
  return new Date(Date.UTC(year, index - 1 + amount, 1)).toISOString().slice(0, 7);
}

export function calendarMonthRange(month: string, today: string): { from: string; to: string } {
  const firstDay = `${month}-01`;
  const lastDay = new Date(Date.UTC(Number(month.slice(0, 4)), Number(month.slice(5, 7)), 0))
    .toISOString().slice(0, 10);
  return { from: month === today.slice(0, 7) && today > firstDay ? today : firstDay, to: lastDay };
}

export function formatCalendarMonth(month: string): string {
  return new Date(`${month}-01T00:00:00.000Z`).toLocaleDateString(undefined, {
    month: "long",
    year: "numeric",
    timeZone: "UTC",
  });
}

export function groupCalendarEntries(entries: CalendarEntry[]): CalendarGroup[] {
  const groups = new Map<string, CalendarEntry[]>();
  for (const entry of entries) {
    const date = entry.episode.air_date?.slice(0, 10);
    if (!date) continue;
    groups.set(date, [...(groups.get(date) ?? []), entry]);
  }
  return [...groups.entries()]
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([date, items]) => ({
      date,
      entries: items.sort((left, right) =>
        left.title.localeCompare(right.title)
        || left.episode.season_number - right.episode.season_number
        || left.episode.episode_number - right.episode.episode_number,
      ),
    }));
}

export function formatCalendarDate(date: string): string {
  // TMDB air dates are civil dates, not instants. Format in UTC to avoid
  // shifting the day when the browser and the user's Visto timezone differ.
  return new Date(`${date}T00:00:00.000Z`).toLocaleDateString(undefined, {
    weekday: "long",
    month: "long",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  });
}
