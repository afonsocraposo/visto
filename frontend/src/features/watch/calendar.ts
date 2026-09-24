import type { CalendarEntry } from "../../types";

export type CalendarGroup = { date: string; entries: CalendarEntry[] };

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
