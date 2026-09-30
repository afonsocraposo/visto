import assert from "node:assert/strict";
import test from "node:test";
import dayjs from "dayjs";
import { formatEpisodeAirDate } from "../src/lib/airDate.ts";

const today = dayjs("2026-09-30T15:30:00");

test("episode air dates within the next week are relative", () => {
  assert.equal(formatEpisodeAirDate("2026-10-01", today), "in 1 day");
  assert.equal(formatEpisodeAirDate("2026-10-02", today), "in 2 days");
  assert.equal(formatEpisodeAirDate("2026-10-06", today), "in 6 days");
  assert.equal(formatEpisodeAirDate("2026-10-07", today), "in 7 days");
});

test("past, current and distant air dates are unchanged", () => {
  assert.equal(formatEpisodeAirDate("2026-09-01", today), "2026-09-01");
  assert.equal(formatEpisodeAirDate("2026-09-30", today), "2026-09-30");
  assert.equal(formatEpisodeAirDate("2026-10-08", today), "2026-10-08");
});

test("missing air dates use the fallback", () => {
  assert.equal(formatEpisodeAirDate("", today), "Air date not announced");
  assert.equal(formatEpisodeAirDate(undefined, today), "Air date not announced");
});
