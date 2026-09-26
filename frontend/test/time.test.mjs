import assert from "node:assert/strict";
import test from "node:test";
import { formatActivityTime } from "../src/lib/time.ts";

const now = new Date("2026-09-24T14:00:00Z");

test("formats recent activity as relative time", () => {
  assert.equal(formatActivityTime("2026-09-24T13:30:00Z", now), "30 minutes ago");
  assert.equal(formatActivityTime("2026-09-23T14:00:00Z", now), "1 day ago");
});

test("uses a calendar date after seven days", () => {
  assert.match(formatActivityTime("2026-09-16T14:00:00Z", now), /Sep 16, 2026/);
});
