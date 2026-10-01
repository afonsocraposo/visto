import assert from "node:assert/strict";
import test from "node:test";
import { emptyResultsTitle, filterByMediaType } from "../src/features/search/discoverMediaType.ts";

const items = [
  { type: "tv", title: "Show" },
  { type: "movie", title: "Film" },
];

test("Given All, When results are filtered, Then every media type is kept in order", () => {
  assert.deepEqual(filterByMediaType(items, "all"), items);
});

test("Given Series or Movies, When results are filtered, Then only that type remains", () => {
  assert.deepEqual(filterByMediaType(items, "tv"), [items[0]]);
  assert.deepEqual(filterByMediaType(items, "movie"), [items[1]]);
});

test("Given a media type, When a search has no matches, Then the empty state names that type", () => {
  assert.equal(emptyResultsTitle("all", "x"), "No results for “x”");
  assert.equal(emptyResultsTitle("tv", "x"), "No series found for “x”");
  assert.equal(emptyResultsTitle("movie", "x"), "No movies found for “x”");
});
