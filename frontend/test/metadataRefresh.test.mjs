import assert from "node:assert/strict";
import test from "node:test";
import {
  canRefreshMetadata,
  metadataRefreshErrorMessage,
  metadataRefreshScopes,
} from "../src/features/details/metadataRefresh.ts";

test("Given an administrator and a saved title, Then metadata can be refreshed", () => {
  assert.equal(canRefreshMetadata("admin", "movie:10"), true);
  assert.equal(canRefreshMetadata("admin", "tv:2316"), true);
});

test("Given a regular user, Then metadata refresh is not offered", () => {
  assert.equal(canRefreshMetadata("user", "movie:10"), false);
});

test("Given a Discover-only title, Then metadata refresh is not offered even to administrators", () => {
  assert.equal(canRefreshMetadata("admin", undefined), false);
  assert.equal(canRefreshMetadata("admin", ""), false);
});

test("Given a movie refresh, Then its detail and library views are refetched", () => {
  assert.deepEqual(metadataRefreshScopes("movie", 10, "movie:10"), [
    ["media-detail", "movie", 10],
    ["library"],
    ["continue"],
  ]);
});

test("Given a TV refresh, Then the rebuilt season and episode catalog is refetched too", () => {
  const scopes = metadataRefreshScopes("tv", 2316, "tv:2316");
  for (const scope of [
    ["media-detail", "tv", 2316],
    ["library"],
    ["continue"],
    ["show-seasons", "tv:2316"],
    ["detail-episodes", "tv:2316"],
    ["detail-all-episodes", "tv:2316"],
  ])
    assert.ok(
      scopes.some((item) => JSON.stringify(item) === JSON.stringify(scope)),
      JSON.stringify(scope),
    );
});

test("Given a failed refresh, Then the message explains what went wrong", () => {
  assert.equal(metadataRefreshErrorMessage({ status: 403 }), "Administrator access required.");
  assert.equal(
    metadataRefreshErrorMessage({ status: 404 }),
    "This title is no longer stored in Visto.",
  );
  assert.equal(
    metadataRefreshErrorMessage({ status: 502 }),
    "Could not refresh metadata from TMDB.",
  );
  assert.equal(
    metadataRefreshErrorMessage(new TypeError("offline")),
    "Could not refresh metadata from TMDB.",
  );
});
