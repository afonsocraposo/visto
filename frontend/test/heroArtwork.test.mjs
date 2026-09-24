import assert from "node:assert/strict";
import test from "node:test";
import { heroArtworkLayers } from "../src/features/details/heroArtwork.ts";

test("Given an episode detail is still loading, When hero art is selected, Then it waits instead of flashing show art", () => {
  assert.deepEqual(heroArtworkLayers(true, null, true, "/show.jpg"), []);
});

test("Given episode art is available, When hero layers are built, Then episode art is above show art with show art as fallback", () => {
  assert.deepEqual(heroArtworkLayers(true, "/episode.jpg", false, "/show.jpg"), ["/episode.jpg", "/show.jpg"]);
});

test("Given an episode has no art after loading, When hero layers are built, Then show art is used as fallback", () => {
  assert.deepEqual(heroArtworkLayers(true, null, false, "/show.jpg"), ["/show.jpg"]);
});

test("Given no episode is selected, When hero layers are built, Then media art is used", () => {
  assert.deepEqual(heroArtworkLayers(false, null, false, "/show.jpg"), ["/show.jpg"]);
});
