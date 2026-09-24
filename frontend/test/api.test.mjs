import test from "node:test";
import assert from "node:assert/strict";

const { api } = await import("../src/lib/api.ts");

test("Given a successful JSON response, When the API client reads it, Then it returns the typed payload", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (path, init) => {
    assert.equal(path, "/api/v1/library");
    assert.deepEqual(init, {});
    return Response.json([{ id: "media-1" }]);
  };
  try {
    assert.deepEqual(await api.get("/api/v1/library"), [{ id: "media-1" }]);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("Given a failed API response, When the API client reads it, Then it uses the server message", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async () => Response.json({ error: "Invalid rating." }, { status: 400 });
  try {
    await assert.rejects(api.patch("/api/v1/library/media-1", { rating: 7 }, "Could not update."), /Invalid rating\./);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("Given an empty success response, When the API client sends a delete, Then it resolves without JSON parsing", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (path, init) => {
    assert.equal(path, "/api/v1/plays/play-1");
    assert.equal(init.method, "DELETE");
    return new Response(null, { status: 204 });
  };
  try {
    assert.equal(await api.delete("/api/v1/plays/play-1"), undefined);
  } finally {
    globalThis.fetch = originalFetch;
  }
});
