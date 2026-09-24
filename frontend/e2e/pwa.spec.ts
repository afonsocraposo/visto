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
  await page.route("**/api/v1/me", route => fulfillJSON(route, user));
  await page.route("**/api/v1/continue-watching", route => fulfillJSON(route, [continueEntry]));
  await page.route("**/api/v1/profile/activity-settings", route => fulfillJSON(route, { activity_visibility: "private", timezone: "Europe/Lisbon" }));
  await page.route("**/api/v1/library", route => fulfillJSON(route, []));
  await page.route("**/api/v1/feed**", route => fulfillJSON(route, { items: [], next_cursor: null }));
  await page.route("**/api/v1/search**", route => fulfillJSON(route, []));
}

test("Given a signed-in user, When they move through the app and choose a theme, Then navigation and the theme update", async ({ page }) => {
  await mockSignedInSession(page);
  await page.goto("/");

  await expect(page.getByRole("button", { name: "Watch" })).toBeVisible();
  await page.getByRole("button", { name: "Feed" }).click();
  await expect(page.getByText("No shared activity yet")).toBeVisible();
  await page.getByRole("button", { name: "Search" }).click();
  await expect(page.getByRole("textbox", { name: "Search TMDB" })).toBeVisible();
  await page.getByRole("button", { name: "Library" }).click();
  await expect(page.getByText("Your library is empty")).toBeVisible();
  await page.getByRole("tab", { name: "Profile & settings" }).click();
  await expect(page.getByText("Choose who can see your activity")).toBeVisible();

  await page.getByRole("combobox", { name: "Color theme" }).click();
  await page.getByRole("option", { name: "Dark" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-mantine-color-scheme", "dark");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("visto-theme"))).toBe("dark");
  await page.getByRole("combobox", { name: "Color theme" }).click();
  await page.getByRole("option", { name: "Light" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-mantine-color-scheme", "light");
});

test("Given the ordinary next episode, When the user taps Watched, Then only that episode is recorded", async ({ page }) => {
  await mockSignedInSession(page);
  let singlePlay: unknown;
  let bulkPlayCount = 0;
  await page.route("**/api/v1/plays", async route => {
    if (route.request().method() === "POST") singlePlay = route.request().postDataJSON();
    await fulfillJSON(route, { id: "play-1" }, 201);
  });
  await page.route("**/api/v1/plays/bulk", async route => {
    bulkPlayCount++;
    await fulfillJSON(route, { items: [] }, 201);
  });
  await page.goto("/");

  await page.getByRole("button", { name: "Watched" }).click();
  await expect.poll(() => singlePlay).toEqual({ episode_id: "tv:42:episode:101" });
  expect(bulkPlayCount).toBe(0);
});

test("Given an installed service worker, When the app shell loads, Then the PWA manifest advertises standalone installation", async ({ browser }) => {
  const context = await browser.newContext({ serviceWorkers: "allow" });
  const page = await context.newPage();
  await page.route("**/api/v1/me", route => fulfillJSON(route, user));
  await page.route("**/api/v1/continue-watching", route => fulfillJSON(route, [continueEntry]));
  try {
    await page.goto("/");
    await expect.poll(() => page.evaluate(async () => (await navigator.serviceWorker.ready).active?.state)).toBe("activated");
    const manifestResponse = await page.request.get("/manifest.webmanifest");
    expect(manifestResponse.ok()).toBeTruthy();
    const manifest = await manifestResponse.json();
    expect(manifest.display).toBe("standalone");
    expect(manifest.icons.some((icon: { src: string }) => icon.src === "/icon.svg")).toBeTruthy();
    expect((await page.request.get("/icon.svg")).ok()).toBeTruthy();
  } finally {
    await context.close();
  }
});
