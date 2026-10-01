import assert from "node:assert/strict";
import test from "node:test";
import dayjs from "dayjs";
import { formatEpisodeAirDate, isUnreleased } from "../src/lib/airDate.ts";

const today = dayjs("2026-09-30T15:30:00");

test("Given an episode airing today or tomorrow, When formatted, Then it reads Today or Tomorrow", () => {
  assert.equal(formatEpisodeAirDate("2026-09-30", today), "Today");
  assert.equal(formatEpisodeAirDate("2026-10-01", today), "Tomorrow");
});

test("Given an episode within the next week, When formatted, Then it counts down in days", () => {
  assert.equal(formatEpisodeAirDate("2026-10-02", today), "In 2 days");
  assert.equal(formatEpisodeAirDate("2026-10-07", today), "In 7 days");
});

test("Given past or distant air dates, When formatted, Then they show a short date with the year only when it differs", () => {
  assert.equal(formatEpisodeAirDate("2026-09-01", today), "Sep 1");
  assert.equal(formatEpisodeAirDate("2026-10-08", today), "Oct 8");
  assert.equal(formatEpisodeAirDate("2025-03-18", today), "Mar 18, 2025");
});

test("Given no air date, When formatted, Then no release state is invented", () => {
  assert.equal(formatEpisodeAirDate("", today), null);
  assert.equal(formatEpisodeAirDate(undefined, today), null);
});

test("Given air dates, When checking release, Then only dated future episodes are unreleased", () => {
  assert.equal(isUnreleased("2026-10-01", today), true);
  assert.equal(isUnreleased("2026-09-30", today), false);
  assert.equal(isUnreleased(null, today), false);
});
