import assert from "node:assert/strict";
import test from "node:test";
import { detailPath } from "../src/features/navigation/detailPath.ts";

test("Given a movie or show, When building its link, Then it points at the media page", () => {
  assert.equal(detailPath({ mediaType: "movie", tmdbID: 7 }), "/media/movie/7");
  assert.equal(detailPath({ mediaType: "tv", tmdbID: 42, seasonNumber: 2 }), "/media/tv/42");
});

test("Given an episode, When building its link, Then it points at the episode page", () => {
  assert.equal(
    detailPath({ mediaType: "tv", tmdbID: 42, seasonNumber: 2, episodeNumber: 7 }),
    "/shows/42/season/2/episode/7",
  );
});
