import assert from "node:assert/strict";
import test from "node:test";
import { activeTabForLocation } from "../src/features/navigation/activeTab.ts";

test("Given a media detail opened from Profile, When the footer tab is resolved, Then Profile stays selected", () => {
  assert.equal(activeTabForLocation("/media/tv/42", "?tab=library"), "library");
});

test("Given an episode detail opened from a show detail in Profile, When the footer tab is resolved, Then Profile stays selected", () => {
  assert.equal(activeTabForLocation("/media/tv/42", "?tab=library&episode=tv%3A42%3Aepisode%3A1"), "library");
});

test("Given a media detail opened from Watching, When the footer tab is resolved, Then Watching stays selected", () => {
  assert.equal(activeTabForLocation("/media/tv/42", "?tab=watch"), "watch");
});

test("Given an actor page opened from a media detail in Profile, When the footer tab is resolved, Then Profile stays selected", () => {
  assert.equal(activeTabForLocation("/people/123", "?tab=library"), "library");
});

test("Given a media detail opened from an actor page in Profile, When the footer tab is resolved, Then Profile stays selected", () => {
  assert.equal(activeTabForLocation("/media/movie/42", "?tab=library"), "library");
});

test("Given a normal Discover page, When the footer tab is resolved, Then Discover is selected", () => {
  assert.equal(activeTabForLocation("/discover", ""), "search");
});

test("Given a media detail with no tab param, When the footer tab is resolved, Then it safely defaults to Watching", () => {
  assert.equal(activeTabForLocation("/media/tv/42", ""), "watch");
});

test("Given a media detail with an invalid tab value, When the footer tab is resolved, Then it safely defaults to Watching", () => {
  assert.equal(activeTabForLocation("/media/tv/42", "?tab=evil"), "watch");
});
