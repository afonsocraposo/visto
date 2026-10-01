import assert from "node:assert/strict";
import test from "node:test";
import { statusAppliesToFilter } from "../src/features/library/mediaFilter.ts";

const statuses = ["watching", "watchlist", "paused", "completed", "dropped"];

test("Given the movie filter, Then only Watchlist and Completed apply", () => {
  assert.deepEqual(
    statuses.filter((status) => statusAppliesToFilter(status, "movie")),
    ["watchlist", "completed"],
  );
});

test("Given the TV or all filter, Then every status applies", () => {
  for (const filter of ["tv", "all"]) {
    assert.deepEqual(
      statuses.filter((status) => statusAppliesToFilter(status, filter)),
      statuses,
    );
  }
});
