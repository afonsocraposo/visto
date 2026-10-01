import assert from "node:assert/strict";
import test from "node:test";
import { groupByLocalDay, relativeDayLabel } from "../src/lib/relativeDay.ts";

const now = new Date(2026, 8, 30, 15, 30);

test("Given today's and yesterday's dates, When labelled, Then they read Today and Yesterday", () => {
  assert.equal(relativeDayLabel("2026-09-30", now), "Today");
  assert.equal(relativeDayLabel("2026-09-29", now), "Yesterday");
});

test("Given an older date, When labelled, Then it shows the day and month, adding the year only when it differs", () => {
  assert.equal(relativeDayLabel("2026-09-28", now), "28 September");
  assert.equal(relativeDayLabel("2025-12-31", now), "31 December 2025");
});

test("Given activity across days, When grouped, Then consecutive items on the same day share a group", () => {
  const items = [new Date(2026, 8, 30, 10), new Date(2026, 8, 30, 8), new Date(2026, 8, 28, 22)];
  const groups = groupByLocalDay(items, (item) => item);
  assert.deepEqual(
    groups.map((group) => [group.day, group.items.length]),
    [
      ["2026-09-30", 2],
      ["2026-09-28", 1],
    ],
  );
});
