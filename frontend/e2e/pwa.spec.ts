import { expect, test, type Page, type Route } from "@playwright/test";

const user = {
  id: "user-1",
  username: "afonso",
  display_name: "Afonso",
  role: "user",
  created_at: "2026-01-01T00:00:00Z",
};

const continueEntry = {
  show_id: "tv:42",
  title: "The Example Show",
  poster_path: "",
  kind: "continue",
  next_episode: {
    id: "tv:42:episode:101",
    season_number: 1,
    episode_number: 1,
    air_date: "2026-09-24",
  },
};

async function fulfillJSON(route: Route, value: unknown, status = 200) {
  await route.fulfill({ status, contentType: "application/json", body: JSON.stringify(value) });
}

async function mockSignedInSession(page: Page) {
  await page.route("**/api/v1/auth/status", (route) =>
    fulfillJSON(route, { bootstrap_available: false, signup_enabled: true, google_enabled: false }),
  );
  await page.route("**/api/v1/me", (route) => fulfillJSON(route, user));
  await page.route("**/api/v1/auth/logout", (route) => route.fulfill({ status: 204, body: "" }));
  await page.route("**/api/v1/public/trending**", (route) =>
    fulfillJSON(route, { tv: [], movies: [] }),
  );
  await page.route("**/api/v1/continue-watching", (route) => fulfillJSON(route, [continueEntry]));
  await page.route("**/api/v1/profile/activity-settings", (route) =>
    fulfillJSON(route, { activity_visibility: "private", timezone: "Europe/Lisbon" }),
  );
  await page.route("**/api/v1/profile/plex-webhook", (route) =>
    fulfillJSON(route, { enabled: false, recent_events: [] }),
  );
  await page.route("**/api/v1/library", (route) => fulfillJSON(route, []));
  await page.route("**/api/v1/feed**", (route) =>
    fulfillJSON(route, { items: [], next_cursor: null }),
  );
  await page.route("**/api/v1/search**", (route) => fulfillJSON(route, []));
}

test("Given a signed-in user, When they navigate and manage appearance and account settings, Then the shell stays compact and actions work", async ({
  page,
}) => {
  await mockSignedInSession(page);
  await page.goto("/");

  await expect(page.locator(".visto-header")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Watching", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Feed" }).click();
  await page.getByRole("tab", { name: "Community" }).click();
  await expect(page.getByText("No shared activity yet")).toBeVisible();
  await page.getByRole("button", { name: "Discover" }).click();
  await expect(page.getByRole("textbox", { name: "Search TMDB" })).toBeVisible();
  await page.getByRole("button", { name: "Profile" }).click();
  await expect(page.getByText("Your library is empty")).toBeVisible();
  await page.getByRole("tab", { name: "Settings" }).click();
  await expect(page.getByText("Choose who can see your activity")).toBeVisible();

  await page.getByRole("combobox", { name: "Color theme" }).click();
  await page.getByRole("option", { name: "Dark" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-mantine-color-scheme", "dark");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("visto-theme"))).toBe("dark");
  await page.getByRole("combobox", { name: "Color theme" }).click();
  await page.getByRole("option", { name: "Light" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-mantine-color-scheme", "light");

  await page.setViewportSize({ width: 390, height: 844 });
  for (const label of ["Watching", "Discover", "Feed", "Profile"]) {
    await expect(page.getByRole("button", { name: label, exact: true })).toBeVisible();
  }
  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page.getByRole("heading", { name: "Welcome to Visto" })).toBeVisible();
});

test("Given a signed-in user, When they open /logout, Then their session ends and they return to sign in", async ({
  page,
}) => {
  let authenticated = true;
  let logoutRequests = 0;
  await page.route("**/api/v1/auth/status", (route) =>
    fulfillJSON(route, { bootstrap_available: false, signup_enabled: true, google_enabled: false }),
  );
  await page.route("**/api/v1/me", (route) =>
    authenticated ? fulfillJSON(route, user) : route.fulfill({ status: 401, body: "" }),
  );
  await page.route("**/api/v1/auth/logout", async (route) => {
    logoutRequests++;
    authenticated = false;
    await route.fulfill({ status: 204, body: "" });
  });
  await page.route("**/api/v1/public/trending**", (route) =>
    fulfillJSON(route, { tv: [], movies: [] }),
  );

  await page.goto("/logout");

  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole("heading", { name: "Welcome to Visto" })).toBeVisible();
  expect(logoutRequests).toBe(1);
});

test("Given Plex sync settings, When the user creates, rotates, and revokes a URL, Then only the current secret URL is shown", async ({
  page,
}) => {
  await mockSignedInSession(page);
  let enabled = false;
  let issueCount = 0;
  const requests: string[] = [];
  await page.route("**/api/v1/profile/plex-webhook", async (route) => {
    const method = route.request().method();
    requests.push(method);
    if (method === "GET") {
      await fulfillJSON(route, { enabled, recent_events: [] });
    } else if (method === "POST") {
      enabled = true;
      issueCount++;
      await fulfillJSON(
        route,
        { webhook_url: `https://visto.example.com/api/v1/webhooks/plex/secret-${issueCount}` },
        201,
      );
    } else if (method === "DELETE") {
      enabled = false;
      await route.fulfill({ status: 204, body: "" });
    }
  });
  await page.goto("/");
  await page.getByRole("button", { name: "Profile" }).click();
  await page.getByRole("tab", { name: "Settings" }).click();
  await page.getByRole("button", { name: "Create webhook URL" }).click();
  await expect(
    page.getByText("https://visto.example.com/api/v1/webhooks/plex/secret-1"),
  ).toBeVisible();

  page.once("dialog", (dialog) => dialog.accept());
  await page.getByRole("button", { name: "Rotate webhook URL" }).click();
  await expect(
    page.getByText("https://visto.example.com/api/v1/webhooks/plex/secret-2"),
  ).toBeVisible();
  await expect(
    page.getByText("https://visto.example.com/api/v1/webhooks/plex/secret-1"),
  ).toHaveCount(0);

  page.once("dialog", (dialog) => dialog.accept());
  await page.getByRole("button", { name: "Revoke" }).click();
  await expect(page.getByRole("button", { name: "Create webhook URL" })).toBeVisible();
  await expect(
    page.getByText("https://visto.example.com/api/v1/webhooks/plex/secret-2"),
  ).toHaveCount(0);
  expect(requests).toContain("DELETE");
});

test("Given an unsaved TV show, When the user adds it or chooses Watch later, Then the selected library status is saved", async ({
  page,
}) => {
  await mockSignedInSession(page);
  const show = {
    tmdb_id: 100,
    type: "tv",
    title: "The Example Show",
    original_title: "The Example Show",
    overview: "A test show.",
    release_date: "2024-01-01",
    poster_path: "",
    original_language: "en",
  };
  await page.route("**/api/v1/search**", (route) => fulfillJSON(route, [show]));
  const savedStatuses: string[] = [];
  await page.route("**/api/v1/library", async (route) => {
    if (route.request().method() === "POST") {
      savedStatuses.push(route.request().postDataJSON().status);
      await fulfillJSON(route, {}, 201);
      return;
    }
    await fulfillJSON(route, []);
  });
  await page.route("**/api/v1/library/**", async (route) => {
    if (route.request().method() === "PATCH") {
      savedStatuses.push(route.request().postDataJSON().status);
      await fulfillJSON(route, {});
      return;
    }
    await fulfillJSON(route, {});
  });
  await page.goto("/");
  await page.getByRole("button", { name: "Discover" }).click();
  await page.getByRole("textbox", { name: "Search TMDB" }).fill("Example");
  await expect(page.getByText("The Example Show").first()).toBeVisible();
  await page.getByRole("button", { name: "Add The Example Show to watching" }).click();
  await expect.poll(() => savedStatuses).toEqual(["watching"]);

  await page.getByRole("button", { name: "Save The Example Show for later" }).click();
  await expect.poll(() => savedStatuses).toEqual(["watching", "watchlist"]);
});

test("Given the ordinary next episode, When the user taps Watched, Then only that episode is recorded", async ({
  page,
}) => {
  await mockSignedInSession(page);
  let singlePlay: unknown;
  let bulkPlayCount = 0;
  await page.route("**/api/v1/plays", async (route) => {
    if (route.request().method() === "POST") singlePlay = route.request().postDataJSON();
    await fulfillJSON(route, { id: "play-1" }, 201);
  });
  await page.route("**/api/v1/plays/bulk", async (route) => {
    bulkPlayCount++;
    await fulfillJSON(route, { items: [] }, 201);
  });
  await page.goto("/");

  await page.locator(".watch-row-action").click();
  await expect.poll(() => singlePlay).toEqual({ episode_id: "tv:42:episode:101" });
  expect(bulkPlayCount).toBe(0);
});

test("Given the Visto server is unreachable, When the user retries after it recovers, Then the connection notice clears and data reloads", async ({
  page,
}) => {
  await mockSignedInSession(page);
  let continueRequests = 0;
  await page.route("**/api/v1/continue-watching", async (route) => {
    continueRequests++;
    if (continueRequests === 1) return route.abort("failed");
    await fulfillJSON(route, [continueEntry]);
  });
  await page.route("**/health", (route) => fulfillJSON(route, { status: "ok" }));
  await page.goto("/");

  const notice = page.getByText(
    "You are offline. Saved information may be out of date, and changes need a connection.",
  );
  await expect(notice).toBeVisible();
  await page.getByRole("button", { name: "Retry connection" }).click();
  await expect(notice).toBeHidden();
  await expect(page.getByText("The Example Show")).toBeVisible();
  expect(continueRequests).toBeGreaterThanOrEqual(2);
});

test("Given an installed service worker, When the app shell loads, Then the PWA manifest advertises standalone installation", async ({
  browser,
}) => {
  const context = await browser.newContext({ serviceWorkers: "allow" });
  const page = await context.newPage();
  await page.route("**/api/v1/me", (route) => fulfillJSON(route, user));
  await page.route("**/api/v1/continue-watching", (route) => fulfillJSON(route, [continueEntry]));
  try {
    await page.goto("/");
    await expect
      .poll(() => page.evaluate(async () => (await navigator.serviceWorker.ready).active?.state))
      .toBe("activated");
    const manifestResponse = await page.request.get("/manifest.webmanifest");
    expect(manifestResponse.ok()).toBeTruthy();
    const manifest = await manifestResponse.json();
    expect(manifest.display).toBe("standalone");
    expect(
      manifest.icons.some((icon: { src: string }) => icon.src === "/icon.svg?v=2"),
    ).toBeTruthy();
    expect((await page.request.get("/icon.svg?v=2")).ok()).toBeTruthy();
  } finally {
    await context.close();
  }
});
