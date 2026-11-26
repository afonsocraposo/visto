import assert from "node:assert/strict";
import test from "node:test";
import { findMissingPriorEpisodes } from "../src/features/library/episodeSelection.ts";

const episode = (id, season, number, airDate, watched = false) => ({
  episode: {
    id,
    show_id: "tv:42",
    season_number: season,
    episode_number: number,
    air_date: airDate,
  },
  name: id,
  watched,
});

test("Given a later season is selected, When prior episodes are checked, Then only released unwatched regular episodes are included", () => {
  const earlierUnwatched = episode("s1e2", 1, 2, "2020-01-08");
  const earlierWatched = episode("s1e1", 1, 1, "2020-01-01", true);
  const special = episode("special", 0, 1, "2019-01-01");
  const notYetReleased = episode("future", 14, 1, "2030-01-01");
  const target = episode("s15e1", 15, 1, "2026-01-01");

  const missing = findMissingPriorEpisodes(
    [earlierUnwatched, earlierWatched, special, notYetReleased, target],
    target,
    "2026-09-24",
  );

  assert.deepEqual(missing, [earlierUnwatched]);
});

test("Given the first episode is selected, When prior episodes are checked, Then no gaps are reported", () => {
  const target = episode("s1e1", 1, 1, "2020-01-01");

  assert.deepEqual(findMissingPriorEpisodes([target], target, "2026-09-24"), []);
});

test("Given only season 2 is being watched, When the next episode is marked, Then no earlier episodes are suggested", () => {
  const target = episode("s2e2", 2, 2, "2020-02-08");
  const entries = [episode("s1e1", 1, 1, "2020-01-01"), episode("s2e1", 2, 1, "2020-02-01", true), target];
  assert.deepEqual(findMissingPriorEpisodes(entries, target, "2026-09-24"), []);
});

test("Given a gap after the last watched episode, When a later episode is marked, Then only the gap is suggested", () => {
  const gap = episode("s2e2", 2, 2, "2020-02-08");
  const target = episode("s2e3", 2, 3, "2020-02-15");
  const entries = [episode("s1e1", 1, 1, "2020-01-01"), episode("s2e1", 2, 1, "2020-02-01", true), gap, target];
  assert.deepEqual(findMissingPriorEpisodes(entries, target, "2026-09-24"), [gap]);
});
