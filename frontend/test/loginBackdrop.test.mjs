import assert from "node:assert/strict";
import test from "node:test";
import { loginBackdropCacheKey, loginBackdropTTL, pickLoginBackdrop, readLoginBackdrop, saveLoginBackdrop, tmdbTitleURL } from "../src/features/auth/loginBackdrop.ts";

const media = (title, type, backdrop_path, tmdb_id = 123) => ({ title, type, backdrop_path, tmdb_id });

test("Given trending media, When choosing a login backdrop, Then only media with wide art is eligible", () => {
  const selection = pickLoginBackdrop({ tv: [media("No art", "tv", ""), media("Show", "tv", "/show.jpg")], movies: [media("Film", "movie", "/film.jpg")] }, 1000, () => 0);
  assert.equal(selection?.title, "Show");
  assert.equal(selection?.tmdb_id, 123);
  assert.equal(selection?.expiresAt, 1000 + loginBackdropTTL);
});

test("Given a selected backdrop, When one minute has not passed, Then it is reused locally", () => {
  const values = new Map();
  const storage = { getItem: key => values.get(key) ?? null, setItem: (key, value) => values.set(key, value) };
  const selection = pickLoginBackdrop({ tv: [media("Show", "tv", "/show.jpg")], movies: [] }, 1000);
  saveLoginBackdrop(storage, selection);
  assert.equal(readLoginBackdrop(storage, 1000 + loginBackdropTTL - 1)?.title, "Show");
  assert.equal(readLoginBackdrop(storage, 1000 + loginBackdropTTL), null);
  assert.ok(values.has(loginBackdropCacheKey));
});

test("Given malformed or unsafe local data, When reading the backdrop, Then it is ignored", () => {
  const storage = { getItem: () => JSON.stringify({ title: "Bad", type: "tv", tmdb_id: 42, backdrop_path: '/bad.jpg");evil', expiresAt: 100000 }) };
  assert.equal(readLoginBackdrop(storage, 1000), null);
});

test("Given cached trending media, When building its TMDB link, Then it uses the matching media type and ID", () => {
  assert.equal(tmdbTitleURL({ type: "tv", tmdb_id: 1622 }), "https://www.themoviedb.org/tv/1622");
  assert.equal(tmdbTitleURL({ type: "movie", tmdb_id: 603 }), "https://www.themoviedb.org/movie/603");
});
