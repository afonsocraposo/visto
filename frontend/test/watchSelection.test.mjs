import assert from "node:assert/strict";
import test from "node:test";
import { regularSeasonsThrough, selectUnwatchedEpisodes, selectWatchedEpisodes } from "../src/features/details/watchSelection.ts";

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

test("Given selected seasons, When a user chooses rewatch or unwatched, Then only watched episodes in those seasons are selected", () => {
	const episodes = [entry("special", 0, true), entry("watched", 1, true), entry("unwatched", 1), entry("other-season", 2, true)];
	assert.deepEqual(selectWatchedEpisodes(episodes, [1]), ["watched"]);
});

test("Given a later season is selected, When previous seasons are included, Then all earlier regular seasons are selected but specials are excluded", () => {
  assert.deepEqual(regularSeasonsThrough([0, 1, 2, 4], 4), [1, 2, 4]);
});
