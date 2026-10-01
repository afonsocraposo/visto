import assert from "node:assert/strict";
import test from "node:test";
import {
  movieReleaseAlertLabel,
  showMovieReleaseAlert,
} from "../src/features/details/movieReleaseAlert.ts";

test("unreleased watchlist movie shows a labeled release alert control", () => {
  assert.equal(showMovieReleaseAlert("watchlist", "2027-02-01", "2026-09-29"), true);
  assert.equal(movieReleaseAlertLabel(true), "Release alert · On");
});

test("released or unsaved movie does not show a release alert control", () => {
  assert.equal(showMovieReleaseAlert("watchlist", "2026-09-29", "2026-09-29"), false);
  assert.equal(showMovieReleaseAlert("watching", "2027-02-01", "2026-09-29"), false);
});
