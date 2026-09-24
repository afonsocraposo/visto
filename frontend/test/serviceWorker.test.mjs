import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import vm from "node:vm";

const origin = "https://visto.test";

function createHarness() {
  const listeners = new Map();
  const cacheStore = new Map();
  const caches = {
    async open(name) {
      if (!cacheStore.has(name)) {
        const entries = new Map();
        cacheStore.set(name, {
          async addAll(urls) {
            for (const url of urls) entries.set(new URL(url, origin).href, new Response("shell"));
          },
          async match(request) { return entries.get(typeof request === "string" ? new URL(request, origin).href : request.url)?.clone(); },
          async put(request, response) { entries.set(request.url, response.clone()); },
          async delete(request) { return entries.delete(typeof request === "string" ? new URL(request, origin).href : request.url); },
          async keys() { return [...entries.keys()].map(url => new Request(url)); },
        });
      }
      return cacheStore.get(name);
    },
    async match(request) {
      for (const cache of cacheStore.values()) {
        const response = await cache.match(request);
        if (response) return response;
      }
      return undefined;
    },
  };
  const self = {
    location: { origin },
    clients: { claim: async () => {} },
    addEventListener: (type, handler) => listeners.set(type, handler),
  };
  let networkFetch = async () => new Response("network");
  const context = vm.createContext({
    self, caches, Request, Response, URL, JSON,
    fetch: request => networkFetch(request),
  });
  const loaded = readFile(new URL("../public/sw.js", import.meta.url), "utf8");

  return {
    cacheStore,
    async start() { vm.runInContext(await loaded, context); },
    setNetworkFetch(fetcher) { networkFetch = fetcher; },
    async dispatchFetch(url, method = "GET") {
      let responsePromise;
      listeners.get("fetch")({
        request: new Request(new URL(url, origin), { method }),
        respondWith(promise) { responsePromise = promise; },
      });
      return responsePromise ? responsePromise : null;
    },
    async install() {
      let installPromise;
      listeners.get("install")({ waitUntil(promise) { installPromise = promise; } });
      await installPromise;
    },
  };
}

test("Given a fresh install, When the service worker installs, Then it precaches the app shell", async () => {
  const harness = createHarness();
  await harness.start();
  await harness.install();

  const shell = harness.cacheStore.get("visto-shell-v1");
  assert.ok(await shell.match(`${origin}/`));
  assert.ok(await shell.match(`${origin}/manifest.webmanifest`));
  assert.ok(await shell.match(`${origin}/icon.svg`));
});

test("Given a browser install prompt, When it reads the web manifest, Then Visto has standalone mode and an app icon", async () => {
  const manifest = JSON.parse(await readFile(new URL("../public/manifest.webmanifest", import.meta.url), "utf8"));
  const html = await readFile(new URL("../index.html", import.meta.url), "utf8");

  assert.equal(manifest.start_url, "/");
  assert.equal(manifest.display, "standalone");
  assert.ok(manifest.icons.some(icon => icon.src && icon.type === "image/svg+xml" && icon.sizes === "any"));
  assert.match(html, /<link\s+rel="manifest"\s+href="\/manifest\.webmanifest"/);
});

test("Given a signed-in user, When an allowed GET is cached, Then offline reads are user-scoped and capped at 100", async () => {
  const harness = createHarness();
  let userID = "alice";
  let online = true;
  harness.setNetworkFetch(async request => {
    if (!online) throw new Error("offline");
    if (new URL(request.url).pathname === "/api/v1/me") {
      return new Response(JSON.stringify({ id: userID }), { status: 200, headers: { "Content-Type": "application/json" } });
    }
    if (new URL(request.url).pathname === "/api/v1/library") {
      return new Response(`${userID}'s library`, { status: 200 });
    }
    return new Response("cached search result", { status: 200 });
  });
  await harness.start();

  await harness.dispatchFetch("/api/v1/me");
  await harness.dispatchFetch("/api/v1/library");
  online = false;
  assert.equal(await (await harness.dispatchFetch("/api/v1/library")).text(), "alice's library");

  // A different account has no access to the previous account's cached body.
  online = true;
  userID = "bob";
  await harness.dispatchFetch("/api/v1/me");
  online = false;
  assert.equal((await harness.dispatchFetch("/api/v1/library")).status, 503);
  assert.equal((await harness.dispatchFetch("/api/v1/library")).headers.get("X-Visto-Offline"), "true");

  // Only successful GETs from the explicit allow-list are retained, with a hard cap.
  online = true;
  userID = "alice";
  await harness.dispatchFetch("/api/v1/me");
  harness.setNetworkFetch(async () => new Response("search", { status: 200 }));
  for (let index = 0; index < 105; index++) {
    await harness.dispatchFetch(`/api/v1/search?q=${index}`);
  }
  assert.equal((await harness.cacheStore.get("visto-user-v1-alice").keys()).length, 100);
});

test("Given an offline client, When a write or failed GET is requested, Then it is not cached or replayed", async () => {
  const harness = createHarness();
  await harness.start();
  harness.setNetworkFetch(async () => new Response("server error", { status: 500 }));

  const failedGet = await harness.dispatchFetch("/api/v1/library");
  assert.equal(failedGet.status, 500);
  assert.equal(await harness.dispatchFetch("/api/v1/library", "POST"), null);
  assert.equal(harness.cacheStore.size, 0);
});

test("Given cached account content, When logout succeeds, Then offline requests cannot reuse that account pointer", async () => {
  const harness = createHarness();
  let online = true;
  harness.setNetworkFetch(async request => {
    if (!online) throw new Error("offline");
    if (new URL(request.url).pathname === "/api/v1/me") {
      return new Response(JSON.stringify({ id: "alice" }), { status: 200 });
    }
    if (new URL(request.url).pathname === "/api/v1/auth/logout") {
      return new Response(null, { status: 204 });
    }
    return new Response("private library", { status: 200 });
  });
  await harness.start();

  await harness.dispatchFetch("/api/v1/me");
  await harness.dispatchFetch("/api/v1/library");
  await harness.dispatchFetch("/api/v1/auth/logout", "POST");
  online = false;

  const response = await harness.dispatchFetch("/api/v1/library");
  assert.equal(response.status, 503);
  assert.equal(response.headers.get("X-Visto-Offline"), "true");
  assert.match(await response.text(), /no cached account data/);
});
