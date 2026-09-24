import test from "node:test";
import assert from "node:assert/strict";

const { resolveMediaID } = await import("../src/features/details/mediaIdentity.ts");

test("Given a media details response with an empty library ID, When a show is added, Then its TMDB media ID is used", () => {
  assert.equal(resolveMediaID(undefined, "", { type: "tv", tmdb_id: 67136 }), "tv:67136");
});

test("Given an existing media ID, When resolving the show ID, Then the existing ID is preserved", () => {
  assert.equal(resolveMediaID("tv:67136", undefined, { type: "tv", tmdb_id: 67136 }), "tv:67136");
});
