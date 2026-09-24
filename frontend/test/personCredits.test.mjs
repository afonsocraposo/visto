import assert from "node:assert/strict";
import test from "node:test";
import { sortPersonCredits } from "../src/features/people/personCredits.ts";

const credit = (tmdb_id, title, type, popularity) => ({ tmdb_id, title, type, popularity });

test("Given mixed movie and TV credits, When actor filmography is sorted, Then the most popular title is first", () => {
  const credits = [credit(1, "Quiet", "movie", 2), credit(2, "Popular Show", "tv", 90), credit(3, "Popular Film", "movie", 90)];
  assert.deepEqual(sortPersonCredits(credits).map(item => item.title), ["Popular Film", "Popular Show", "Quiet"]);
  assert.deepEqual(credits.map(item => item.title), ["Quiet", "Popular Show", "Popular Film"]);
});
