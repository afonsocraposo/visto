import test from "node:test";
import assert from "node:assert/strict";

const { APIRequestError, api, retryTransientRequest } = await import("../src/lib/api.ts");

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
    await assert.rejects(
      api.patch("/api/v1/library/media-1", { rating: 7 }, "Could not update."),
      /Invalid rating\./,
    );
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("Given an API failure, When retry policy checks it, Then only transient responses are retried", async () => {
  assert.equal(retryTransientRequest(0, new APIRequestError("Busy", 503)), true);
  assert.equal(retryTransientRequest(0, new APIRequestError("Slow down", 429)), true);
  assert.equal(retryTransientRequest(0, new APIRequestError("Not found", 404)), false);
  assert.equal(retryTransientRequest(2, new APIRequestError("Busy", 503)), false);
  assert.equal(retryTransientRequest(0, new TypeError("Failed to fetch")), true);
});

test("Given a failed network request, When the API client cannot reach the server, Then it reports the unavailable connection", async () => {
  const originalFetch = globalThis.fetch;
  const originalWindow = globalThis.window;
  const events = [];
  globalThis.window = { dispatchEvent: (event) => events.push(event.type) };
  globalThis.fetch = async () => {
    throw new TypeError("Failed to fetch");
  };
  try {
    await assert.rejects(api.get("/api/v1/library"), /Failed to fetch/);
    assert.deepEqual(events, ["visto:connection-unavailable"]);
  } finally {
    globalThis.fetch = originalFetch;
    globalThis.window = originalWindow;
  }
});

test("Given a service-worker offline response, When the API client reads it, Then it reports the unavailable connection", async () => {
  const originalFetch = globalThis.fetch;
  const originalWindow = globalThis.window;
  const events = [];
  globalThis.window = { dispatchEvent: (event) => events.push(event.type) };
  globalThis.fetch = async () =>
    new Response(JSON.stringify({ error: "offline" }), {
      status: 503,
      headers: { "Content-Type": "application/json", "X-Visto-Offline": "true" },
    });
  try {
    await assert.rejects(api.get("/api/v1/library"), /offline/);
    assert.deepEqual(events, ["visto:connection-unavailable"]);
  } finally {
    globalThis.fetch = originalFetch;
    globalThis.window = originalWindow;
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

test("Given episode IDs, When the API client deletes with a body, Then it sends the JSON payload", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (path, init) => {
    assert.equal(path, "/api/v1/plays/bulk");
    assert.equal(init.method, "DELETE");
    assert.deepEqual(JSON.parse(init.body), { episode_ids: ["episode-1", "episode-2"] });
    return new Response(null, { status: 204 });
  };
  try {
    assert.equal(
      await api.delete("/api/v1/plays/bulk", "Could not mark unwatched.", {
        episode_ids: ["episode-1", "episode-2"],
      }),
      undefined,
    );
  } finally {
    globalThis.fetch = originalFetch;
  }
});
