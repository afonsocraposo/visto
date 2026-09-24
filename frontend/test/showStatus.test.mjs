import assert from "node:assert/strict";
import test from "node:test";
import { hasCaughtUpDisplayState } from "../src/features/library/showStatus.ts";

test("Given released episodes with no later unwatched release, When progress has a cursor, Then the show can display caught up", () => {
  assert.equal(hasCaughtUpDisplayState({ cursor: { id: "s15e10", season_number: 15, episode_number: 10, air_date: "2026-09-01" }, is_caught_up: true }), true);
});

test("Given a show with no watched released episode, When progress has no cursor, Then it is not labeled caught up", () => {
  assert.equal(hasCaughtUpDisplayState({ cursor: null, is_caught_up: true }), false);
});

test("Given a later released episode exists, When progress is not caught up, Then the caught-up label is hidden", () => {
  assert.equal(hasCaughtUpDisplayState({ cursor: { id: "s1e1", season_number: 1, episode_number: 1, air_date: "2020-01-01" }, is_caught_up: false }), false);
});
