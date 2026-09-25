import assert from "node:assert/strict";
import test from "node:test";
import { heroArtworkLayers, resolveMediaArtwork } from "../src/features/details/heroArtwork.ts";

test("keeps the hero on its gradient while a preferred media backdrop is loading", () => {
  assert.equal(resolveMediaArtwork(null, "/poster.jpg", true), null);
});

test("shows the preferred backdrop immediately when it is available", () => {
  assert.equal(resolveMediaArtwork("/backdrop.jpg", "/poster.jpg", true), "/backdrop.jpg");
});

test("uses the poster only after the backdrop request is complete without art", () => {
  assert.equal(resolveMediaArtwork(null, "/poster.jpg", false), "/poster.jpg");
});

test("Given an episode detail is still loading, When hero art is selected, Then it waits instead of flashing show art", () => {
  assert.deepEqual(heroArtworkLayers(true, null, true, "/show.jpg"), []);
});

test("Given episode art is available, When hero layers are built, Then episode art is above show art with show art as fallback", () => {
  assert.deepEqual(heroArtworkLayers(true, "/episode.jpg", false, "/show.jpg"), [
    "/episode.jpg",
    "/show.jpg",
  ]);
});

test("Given an episode has no art after loading, When hero layers are built, Then show art is used as fallback", () => {
  assert.deepEqual(heroArtworkLayers(true, null, false, "/show.jpg"), ["/show.jpg"]);
});

test("Given no episode is selected, When hero layers are built, Then media art is used", () => {
  assert.deepEqual(heroArtworkLayers(false, null, false, "/show.jpg"), ["/show.jpg"]);
});
