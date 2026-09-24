import test from "node:test";
import assert from "node:assert/strict";
import { formatCalendarDate, groupCalendarEntries } from "../src/features/watch/calendar.ts";

const entry = (id, title, date, season = 1, episode = 1) => ({
  show_id: `tv:${id}`,
  title,
  episode: { id: `episode-${id}-${season}-${episode}`, season_number: season, episode_number: episode, air_date: date },
});

test("Given unordered upcoming episodes, When grouped for the calendar, Then dates and episodes are ordered", () => {
  const groups = groupCalendarEntries([
    entry(1, "Zeta", "2026-10-02"),
    entry(2, "Alpha", "2026-10-01", 2, 1),
    entry(3, "Alpha", "2026-10-01", 1, 4),
  ]);

  assert.deepEqual(groups.map(group => group.date), ["2026-10-01", "2026-10-02"]);
  assert.deepEqual(groups[0].entries.map(item => item.episode.id), ["episode-3-1-4", "episode-2-2-1"]);
});

test("Given a date-only TMDB air date, When it is formatted, Then the day does not shift with the device timezone", () => {
  const formatted = formatCalendarDate("2026-10-01");
  assert.match(formatted, /October 1, 2026/);
});
