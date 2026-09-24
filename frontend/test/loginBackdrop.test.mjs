import assert from "node:assert/strict";
import test from "node:test";
import { loginBackdropCacheKey, loginBackdropTTL, pickLoginBackdrop, readLoginBackdrop, saveLoginBackdrop } from "../src/features/auth/loginBackdrop.ts";

const media = (title, type, backdrop_path) => ({ title, type, backdrop_path });

test("Given trending media, When choosing a login backdrop, Then only media with wide art is eligible", () => {
  const selection = pickLoginBackdrop({ tv: [media("No art", "tv", ""), media("Show", "tv", "/show.jpg")], movies: [media("Film", "movie", "/film.jpg")] }, 1000, () => 0);
  assert.equal(selection?.title, "Show");
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
  const storage = { getItem: () => JSON.stringify({ title: "Bad", type: "tv", backdrop_path: '/bad.jpg");evil', expiresAt: 100000 }) };
  assert.equal(readLoginBackdrop(storage, 1000), null);
});
