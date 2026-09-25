import test from "node:test";
import assert from "node:assert/strict";
import {
  calendarMonthRange,
  dateInTimezone,
  formatCalendarDate,
  groupCalendarEntries,
  monthInTimezone,
  shiftCalendarMonth,
} from "../src/features/watch/calendar.ts";

const entry = (id, title, date, season = 1, episode = 1) => ({
  show_id: `tv:${id}`,
  title,
  episode: {
    id: `episode-${id}-${season}-${episode}`,
    season_number: season,
    episode_number: episode,
    air_date: date,
  },
});

test("Given unordered upcoming episodes, When grouped for the calendar, Then dates and episodes are ordered", () => {
  const groups = groupCalendarEntries([
    entry(1, "Zeta", "2026-10-02"),
    entry(2, "Alpha", "2026-10-01", 2, 1),
    entry(3, "Alpha", "2026-10-01", 1, 4),
  ]);

  assert.deepEqual(
    groups.map((group) => group.date),
    ["2026-10-01", "2026-10-02"],
  );
  assert.deepEqual(
    groups[0].entries.map((item) => item.episode.id),
    ["episode-3-1-4", "episode-2-2-1"],
  );
});

test("Given a date-only TMDB air date, When it is formatted, Then the day does not shift with the device timezone", () => {
  const formatted = formatCalendarDate("2026-10-01");
  assert.match(formatted, /October 1, 2026/);
});

test("Given an instant near midnight, When the user's timezone is applied, Then the calendar uses that local date", () => {
  const instant = new Date("2026-09-24T23:30:00.000Z");
  assert.equal(dateInTimezone("Europe/Lisbon", instant), "2026-09-25");
  assert.equal(monthInTimezone("America/Los_Angeles", instant), "2026-09");
});

test("Given a selected calendar month, When its range is computed, Then the current month starts today and other months use full boundaries", () => {
  assert.deepEqual(calendarMonthRange("2026-09", "2026-09-24"), {
    from: "2026-09-24",
    to: "2026-09-30",
  });
  assert.deepEqual(calendarMonthRange("2026-10", "2026-09-24"), {
    from: "2026-10-01",
    to: "2026-10-31",
  });
  assert.deepEqual(calendarMonthRange("2028-02", "2028-02-29"), {
    from: "2028-02-29",
    to: "2028-02-29",
  });
});

test("Given a month boundary, When the user navigates the calendar, Then month navigation handles year changes", () => {
  assert.equal(shiftCalendarMonth("2026-12", 1), "2027-01");
  assert.equal(shiftCalendarMonth("2027-01", -1), "2026-12");
});
