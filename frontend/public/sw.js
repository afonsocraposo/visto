const shell = "visto-shell-v1";
const userIndex = "visto-user-index-v1";
const activeUserRequest = new Request(`${self.location.origin}/__visto_active_user__`);
self.addEventListener("install", event => event.waitUntil(caches.open(shell).then(cache => cache.addAll(["/", "/manifest.webmanifest", "/icon.svg"]))));
self.addEventListener("activate", event => event.waitUntil(self.clients.claim()));
self.addEventListener("fetch", event => {
  const url = new URL(event.request.url);
  if (event.request.method === "POST" && url.pathname === "/api/v1/auth/login") {
    event.respondWith(fetch(event.request).then(async response => { if (response.ok) await storeActiveUser(response.clone()); return response; }));
    return;
  }
  if (event.request.method !== "GET") return;
  if (url.pathname.startsWith("/api/")) {
    event.respondWith(fetch(event.request).then(async response => {
      if (url.pathname === "/api/v1/me" && response.ok) {
        const user = await response.clone().json();
        if (user.id) {
          const cache = await caches.open(userCacheName(user.id));
          await cache.put(event.request, response.clone());
          await storeActiveUserID(user.id);
        }
        return response;
      }
      if (!response.ok || url.pathname.startsWith("/api/v1/export/")) return response;
      const userID = await activeUserID();
      if (userID) await (await caches.open(userCacheName(userID))).put(event.request, response.clone());
      return response;
    }).catch(async () => {
      const userID = await activeUserID();
      if (!userID) return new Response(JSON.stringify({ error: "offline and no cached account data" }), { status: 503, headers: { "Content-Type": "application/json" } });
      return (await caches.open(userCacheName(userID))).match(event.request) || new Response(JSON.stringify({ error: "offline and content is not cached" }), { status: 503, headers: { "Content-Type": "application/json" } });
    }));
    return;
  }
  if (event.request.mode === "navigate") event.respondWith(fetch(event.request).catch(() => caches.match("/")));
});

function userCacheName(userID) { return `visto-user-v1-${encodeURIComponent(userID)}`; }
async function activeUserID() { const response = await (await caches.open(userIndex)).match(activeUserRequest); return response ? response.text() : null; }
async function storeActiveUserID(userID) { await (await caches.open(userIndex)).put(activeUserRequest, new Response(userID)); }
async function storeActiveUser(response) { try { const user = await response.json(); if (user.id) await storeActiveUserID(user.id); } catch {} }
