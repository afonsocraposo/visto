import assert from "node:assert/strict";
import test from "node:test";
import { swipeDirection } from "../src/features/details/episodeNavigation.ts";

const W = 390;

test("swiping left goes to the next episode and right to the previous", () => {
  assert.equal(swipeDirection([300, 400], [180, 410], W), "next");
  assert.equal(swipeDirection([150, 400], [280, 390], W), "previous");
});

test("swipes starting at the screen edges are left to the OS back gesture", () => {
  assert.equal(swipeDirection([10, 400], [200, 400], W), null);
  assert.equal(swipeDirection([380, 400], [200, 400], W), null);
  assert.equal(swipeDirection([32, 400], [200, 400], W), "previous");
});

test("short or mostly vertical movements are ignored", () => {
  assert.equal(swipeDirection([200, 400], [160, 400], W), null);
  assert.equal(swipeDirection([200, 400], [120, 520], W), null);
});
