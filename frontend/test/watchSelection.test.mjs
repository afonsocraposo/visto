import assert from "node:assert/strict";
import test from "node:test";
import { selectUnwatchedEpisodes } from "../src/features/details/watchSelection.ts";

const entry = (id, season, watched = false, airDate = "2026-09-01") => ({
  episode: { id, season_number: season, episode_number: 1, air_date: airDate },
  name: id,
  watched,
});

test("Given a show with specials and unreleased episodes, When regular seasons are selected, Then only released unwatched regular episodes are included", () => {
  const episodes = [entry("special", 0), entry("watched", 1, true), entry("regular", 1), entry("future", 2, false, "2026-12-01"), entry("season-two", 2)];
  assert.deepEqual(selectUnwatchedEpisodes(episodes, [1, 2], "2026-09-24"), ["regular", "season-two"]);
});

test("Given specials are selected, When a show is marked watched, Then specials are included", () => {
  assert.deepEqual(selectUnwatchedEpisodes([entry("special", 0), entry("regular", 1)], [0, 1], "2026-09-24"), ["special", "regular"]);
});
