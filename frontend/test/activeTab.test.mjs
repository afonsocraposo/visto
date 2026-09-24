import assert from "node:assert/strict";
import test from "node:test";
import { activeTabForLocation } from "../src/features/navigation/activeTab.ts";

const origin = "http://visto.test";

test("Given a media detail opened from Profile, When the footer tab is resolved, Then Profile stays selected", () => {
  assert.equal(activeTabForLocation("/media/tv/42", "?from=%2Fprofile", origin), "library");
});

test("Given an episode detail opened from a show detail in Profile, When the footer tab is resolved, Then Profile stays selected", () => {
  const showDetail = `/media/tv/42?from=${encodeURIComponent("/profile")}`;
  const episodeDetailSearch = `?from=${encodeURIComponent(showDetail)}`;
  assert.equal(activeTabForLocation("/media/tv/42", episodeDetailSearch, origin), "library");
});

test("Given a media detail opened from Watching, When the footer tab is resolved, Then Watching stays selected", () => {
  assert.equal(activeTabForLocation("/media/tv/42", "?from=%2Fwatch", origin), "watch");
});

test("Given a normal Discover page, When the footer tab is resolved, Then Discover is selected", () => {
  assert.equal(activeTabForLocation("/discover", "", origin), "search");
});

test("Given an external return URL, When the footer tab is resolved, Then it safely defaults to Watching", () => {
  assert.equal(activeTabForLocation("/media/tv/42", "?from=https%3A%2F%2Fevil.test%2Fprofile", origin), "watch");
});
