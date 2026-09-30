import assert from "node:assert/strict";
import test from "node:test";
import { episodePosition } from "../src/lib/episodePosition.ts";

test("episodePosition parses episode labels", () => {
  assert.deepEqual(episodePosition("S02E06"), { seasonNumber: 2, episodeNumber: 6 });
  assert.deepEqual(episodePosition("Movie"), {});
  assert.deepEqual(episodePosition(undefined), {});
});
