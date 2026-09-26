import assert from "node:assert/strict";
import test from "node:test";
import { loadSession } from "../src/features/auth/session.ts";

test("session lookup treats only 401 as signed out", async () => {
  const originalFetch = globalThis.fetch;
  try {
    globalThis.fetch = async () => new Response(null, { status: 401 });
    assert.equal(await loadSession(), null);

    globalThis.fetch = async () => new Response('{"id":"user-1"}', { status: 200 });
    assert.equal((await loadSession()).id, "user-1");

    globalThis.fetch = async () => new Response(null, { status: 500 });
    await assert.rejects(loadSession(), /Could not check your session/);
  } finally {
    globalThis.fetch = originalFetch;
  }
});
