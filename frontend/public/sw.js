const shell = "visto-shell-v1";
const userIndex = "visto-user-index-v1";
const maxCachedAPIEntries = 100;
const cacheableAPIPaths = new Set([
  "/api/v1/me",
  "/api/v1/search",
  "/api/v1/library",
  "/api/v1/plays",
  "/api/v1/feed",
  "/api/v1/continue-watching",
  "/api/v1/calendar",
  "/api/v1/profile/activity-settings",
]);
const activeUserRequest = new Request(`${self.location.origin}/__visto_active_user__`);
self.addEventListener("install", event => event.waitUntil(caches.open(shell).then(cache => cache.addAll(["/", "/manifest.webmanifest", "/icon.svg"]))));
self.addEventListener("activate", event => event.waitUntil(self.clients.claim()));
self.addEventListener("fetch", event => {
  const url = new URL(event.request.url);
  if (event.request.method === "POST" && url.pathname === "/api/v1/auth/login") {
    event.respondWith(fetch(event.request).then(async response => { if (response.ok) await storeActiveUser(response.clone()); return response; }));
    return;
  }
  if (event.request.method === "POST" && url.pathname === "/api/v1/auth/logout") {
    event.respondWith(fetch(event.request).then(async response => { if (response.ok) await clearActiveUserData(); return response; }));
    return;
  }
  if (event.request.method !== "GET") return;
  if (url.pathname.startsWith("/api/") && isCacheableAPIPath(url.pathname)) {
    event.respondWith(fetch(event.request).then(async response => {
      if (url.pathname === "/api/v1/me" && response.ok) {
        const user = await response.clone().json();
        if (user.id) {
          await cacheRecentResponse(user.id, event.request, response.clone());
          await storeActiveUserID(user.id);
        }
        return response;
      }
      if (response.status !== 200) return response;
      const userID = await activeUserID();
      if (userID) await cacheRecentResponse(userID, event.request, response.clone());
      return response;
    }).catch(async () => {
      const userID = await activeUserID();
      if (!userID) return offlineResponse("offline and no cached account data");
      const cachedResponse = await (await caches.open(userCacheName(userID))).match(event.request);
      return cachedResponse || offlineResponse("offline and content is not cached");
    }));
    return;
  }
  if (event.request.mode === "navigate") event.respondWith(fetch(event.request).catch(() => caches.match("/")));
});

function offlineResponse(message) {
  return new Response(JSON.stringify({ error: message }), {
    status: 503,
    headers: { "Content-Type": "application/json", "X-Visto-Offline": "true" },
  });
}
function userCacheName(userID) { return `visto-user-v1-${encodeURIComponent(userID)}`; }
function isCacheableAPIPath(path) {
  return cacheableAPIPaths.has(path)
    || /^\/api\/v1\/(movies|shows)\/\d+$/.test(path)
    || /^\/api\/v1\/shows\/[^/]+\/(episodes|seasons|progress)$/.test(path)
    || /^\/api\/v1\/seasons\/[^/]+\/episodes$/.test(path);
}
async function activeUserID() { const response = await (await caches.open(userIndex)).match(activeUserRequest); return response ? response.text() : null; }
async function storeActiveUserID(userID) { await (await caches.open(userIndex)).put(activeUserRequest, new Response(userID)); }
async function cacheRecentResponse(userID, request, response) {
  const cache = await caches.open(userCacheName(userID));
  await cache.delete(request);
  await cache.put(request, response);
  const requests = await cache.keys();
  for (const staleRequest of requests.slice(0, Math.max(0, requests.length - maxCachedAPIEntries))) {
    await cache.delete(staleRequest);
  }
}
async function clearActiveUserID() { await (await caches.open(userIndex)).delete(activeUserRequest); }
async function clearActiveUserData() {
  const userID = await activeUserID();
  if (userID) await caches.delete(userCacheName(userID));
  await clearActiveUserID();
}
async function storeActiveUser(response) { try { const user = await response.json(); if (user.id) await storeActiveUserID(user.id); } catch {} }
