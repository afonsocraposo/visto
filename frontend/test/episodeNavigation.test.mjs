import assert from "node:assert/strict";
import test from "node:test";
import { dragFollow, getAdjacentEpisodes } from "../src/features/details/episodeNavigation.ts";

const episode = (season, number, { watched = false, id = `s${season}e${number}` } = {}) => ({
  episode: {
    id,
    season_number: season,
    episode_number: number,
    air_date: null,
  },
  name: id,
  watched,
});

const adjacent = (current, entries) => {
  const result = getAdjacentEpisodes(current.episode, entries);
  return [result.previousEpisode?.episode.id ?? null, result.nextEpisode?.episode.id ?? null];
};

test("finds the episodes before and after a middle episode", () => {
  const entries = [episode(1, 2), episode(1, 1), episode(1, 3)];
  assert.deepEqual(adjacent(entries[0], entries), ["s1e1", "s1e3"]);
});

test("handles the first and last episodes", () => {
  const entries = [episode(1, 1), episode(1, 2)];
  assert.deepEqual(adjacent(entries[0], entries), [null, "s1e2"]);
  assert.deepEqual(adjacent(entries[1], entries), ["s1e1", null]);
});

test("crosses seasons in both directions, including multi-season shows", () => {
  const entries = [episode(2, 1), episode(1, 10), episode(3, 1)];
  assert.deepEqual(adjacent(entries[1], entries), [null, "s2e1"]);
  assert.deepEqual(adjacent(entries[0], entries), ["s1e10", "s3e1"]);
});

test("ignores specials for regular episodes and keeps special navigation within season 0", () => {
  const entries = [episode(1, 1), episode(0, 1), episode(0, 2), episode(1, 2)];
  assert.deepEqual(adjacent(entries[0], entries), [null, "s1e2"]);
  assert.deepEqual(adjacent(entries[1], entries), [null, "s0e2"]);
  assert.deepEqual(adjacent(entries[2], entries), ["s0e1", null]);
});

test("watch state does not change episode order", () => {
  const entries = [
    episode(1, 1, { watched: true }),
    episode(1, 2),
    episode(1, 3, { watched: true }),
  ];
  assert.deepEqual(adjacent(entries[1], entries), ["s1e1", "s1e3"]);
});

test("Given an 80px drag toward an episode, When following the finger, Then content moves ~24px and dims slightly", () => {
  const { shift, opacity } = dragFollow(-80, true);
  assert.equal(shift, -24);
  assert.ok(opacity < 1 && opacity >= 0.88);
});

test("Given a long drag, When following the finger, Then the shift is capped", () => {
  assert.equal(dragFollow(400, true).shift, 28);
});

test("Given no episode in that direction, When dragging, Then the content barely moves and keeps full opacity", () => {
  const { shift, opacity } = dragFollow(80, false);
  assert.ok(Math.abs(shift) < 10);
  assert.equal(opacity, 1);
});
