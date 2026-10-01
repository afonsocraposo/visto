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

async function touchDrag(page: Page, from: { x: number; y: number }, to: { x: number; y: number }) {
  const cdp = await page.context().newCDPSession(page);
  const send = (type: string, point?: { x: number; y: number }) =>
    cdp.send("Input.dispatchTouchEvent", { type, touchPoints: point ? [point] : [] });
  await send("touchStart", from);
  const steps = 8;
  for (let i = 1; i <= steps; i++)
    await send("touchMove", {
      x: from.x + ((to.x - from.x) * i) / steps,
      y: from.y + ((to.y - from.y) * i) / steps,
    });
  await send("touchEnd");
  await cdp.detach();
}

async function mockSignedInSession(page: Page, asAdmin = false) {
  let signedIn = true;
  await page.route("**/api/v1/auth/status", (route) =>
    fulfillJSON(route, { bootstrap_available: false, signup_enabled: true, google_enabled: false }),
  );
  await page.route("**/api/v1/me", (route) =>
    signedIn
      ? fulfillJSON(route, { ...user, role: asAdmin ? "admin" : "user" })
      : fulfillJSON(route, { error: "signed out" }, 401),
  );
  await page.route("**/api/v1/imports/welcome", (route) => fulfillJSON(route, { pending: false }));
  await page.route("**/api/v1/auth/logout", (route) => {
    signedIn = false;
    return route.fulfill({ status: 204, body: "" });
  });
  await page.route("**/api/v1/public/trending**", (route) =>
    fulfillJSON(route, { tv: [], movies: [] }),
  );
  await page.route("**/api/v1/continue-watching", (route) => fulfillJSON(route, [continueEntry]));
  await page.route("**/api/v1/profile/activity-settings", (route) =>
    fulfillJSON(route, { activity_visibility: "private", timezone: "Europe/Lisbon" }),
  );
  await page.route("**/api/v1/profile/plex-webhook", (route) =>
    fulfillJSON(route, {
      mode: "personal",
      enabled: false,
      managed_webhook_enabled: false,
      recent_events: [],
    }),
  );
  await page.route(/\/api\/v1\/library(?:\?.*)?$/, (route) =>
    fulfillJSON(route, { items: [], next_cursor: null }),
  );
  await page.route("**/api/v1/library/lookup", (route) => fulfillJSON(route, []));
  await page.route("**/api/v1/feed**", (route) =>
    fulfillJSON(route, { items: [], next_cursor: null }),
  );
  await page.route("**/api/v1/search**", (route) => fulfillJSON(route, []));
  await page.route("**/api/v1/shows/**/seasons", (route) =>
    fulfillJSON(route, [{ id: "tv:100:season:1", season_number: 1, episode_count: 2 }]),
  );
  await page.route("**/api/v1/seasons/**/episodes", (route) =>
    fulfillJSON(route, { items: [], next_cursor: null }),
  );
}

test("Given a phone viewport, When switching destinations, Then the bottom navigation stays usable", async ({
  page,
}) => {
  await mockSignedInSession(page);
  const nav = page.getByRole("navigation", { name: "Main navigation" });

  for (const width of [320, 390]) {
    await page.setViewportSize({ width, height: 740 });
    await page.goto("/watch");
    const navBounds = await nav.evaluate((element) => {
      const { height, bottom } = element.getBoundingClientRect();
      return { height, bottom };
    });
    expect(navBounds).toEqual({ height: 50, bottom: 740 });
    await expect(nav.getByRole("link", { name: "Watching" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    const boxes = await nav.getByRole("link").evaluateAll((buttons) =>
      buttons.map((button) => {
        const { x, width, height } = button.getBoundingClientRect();
        return { x, width, height, direction: getComputedStyle(button).flexDirection };
      }),
    );
    expect(boxes).toHaveLength(4);
    expect(
      boxes.every((box) => box.width >= 44 && box.height >= 44 && box.direction === "column"),
    ).toBe(true);
    expect(boxes[0].x).toBeGreaterThanOrEqual(0);
    expect(boxes[3].x + boxes[3].width).toBeLessThanOrEqual(width);

    await nav.getByRole("link", { name: "Discover" }).click();
    await expect(nav.getByRole("link", { name: "Discover" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(
      await nav.evaluate((element) => {
        const { height, bottom } = element.getBoundingClientRect();
        return { height, bottom };
      }),
    ).toEqual(navBounds);
  }
});

test("Main navigation supports opening a destination in a new tab", async ({ page }) => {
  await mockSignedInSession(page);
  await page.setViewportSize({ width: 390, height: 740 });
  await page.goto("/watch");
  const discover = page.locator(".bottom-nav-button", { hasText: "Discover" });
  await expect(discover).toHaveAttribute("href", "/discover");
  const popupPromise = page.context().waitForEvent("page");
  await discover.click({ modifiers: ["ControlOrMeta"] });
  const popup = await popupPromise;
  await expect(popup).toHaveURL(/\/discover$/);
  await popup.close();
  await expect(page).toHaveURL(/\/watch$/);
});

test("Given an iPhone safe-area inset, Then the bottom bar stays compact and usable", async ({
  page,
}) => {
  await mockSignedInSession(page);
  await page.setViewportSize({ width: 390, height: 740 });
  const cdp = await page.context().newCDPSession(page);
  await cdp.send("Emulation.setSafeAreaInsetsOverride", { insets: { bottom: 34 } });
  await page.goto("/watch");

  const layout = await page.locator(".visto-footer").evaluate((footer) => {
    const nav = footer.querySelector(".bottom-nav");
    if (!nav) throw new Error("Bottom navigation is missing");
    const footerBounds = footer.getBoundingClientRect();
    return {
      footerHeight: footerBounds.height,
      footerBottom: footerBounds.bottom,
      navHeight: nav.getBoundingClientRect().height,
      navBottom: nav.getBoundingClientRect().bottom,
      paddingBottom: parseFloat(getComputedStyle(footer).paddingBottom),
      mainPaddingBottom: parseFloat(
        getComputedStyle(document.querySelector(".visto-main")!).paddingBottom,
      ),
      buttons: Array.from(nav.querySelectorAll("a"), (button) => {
        const { width, height } = button.getBoundingClientRect();
        return { width, height };
      }),
    };
  });
  expect(layout.footerHeight).toBe(84);
  expect(layout.footerBottom).toBe(740);
  expect(layout.navHeight).toBe(50);
  expect(layout.navBottom).toBe(706);
  expect(layout.paddingBottom).toBe(34);
  expect(layout.mainPaddingBottom).toBe(116);
  expect(layout.buttons).toHaveLength(4);
  expect(layout.buttons.every(({ width, height }) => width >= 44 && height >= 44)).toBe(true);
  await cdp.detach();
});

test("Given a signed-in user, When they navigate and manage appearance and account settings, Then the shell stays compact and actions work", async ({
  page,
}) => {
  await mockSignedInSession(page);
  await page.goto("/");

  await expect(page.locator(".visto-header")).toHaveCount(0);
  await expect(page.getByRole("link", { name: "Watching", exact: true })).toBeVisible();
  await page.getByRole("tab", { name: "Upcoming" }).click();
  await expect
    .poll(() => page.evaluate(() => sessionStorage.getItem("visto:tab:user-1:watching")))
    .toBe("calendar");
  await page.getByRole("link", { name: "Activity", exact: true }).click();
  await page.getByRole("tab", { name: "Community" }).click();
  await expect(page.getByText("No shared activity yet")).toBeVisible();
  await expect
    .poll(() => page.evaluate(() => localStorage.getItem("visto:tab:user-1:feed")))
    .toBe("community");
  await page.getByRole("link", { name: "Discover" }).click();
  await expect(page.getByRole("textbox", { name: "Search TMDB" })).toBeVisible();
  await page.getByRole("link", { name: "Library" }).click();
  await expect(page.getByRole("heading", { name: "Start with your watch history" })).toBeVisible();
  await page.getByRole("link", { name: "Account and settings" }).click();
  await expect(page.getByText("Choose who can see your activity")).toBeVisible();
  await expect(page).toHaveURL(/\/settings$/);

  await page.getByRole("combobox", { name: "Color theme" }).click();
  await page.getByRole("option", { name: "Dark" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-mantine-color-scheme", "dark");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("visto-theme"))).toBe("dark");
  await page.getByRole("combobox", { name: "Color theme" }).click();
  await page.getByRole("option", { name: "Light" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-mantine-color-scheme", "light");
  await page.getByRole("link", { name: "Watching", exact: true }).click();
  await expect(page.getByRole("tab", { name: "Upcoming" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await page.getByRole("link", { name: "Activity", exact: true }).click();
  await expect(page.getByRole("tab", { name: "Community" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await page.reload();
  await page.getByRole("link", { name: "Activity", exact: true }).click();
  await expect(page.getByRole("tab", { name: "Community" })).toHaveAttribute(
    "aria-selected",
    "true",
  );

  await page.setViewportSize({ width: 390, height: 844 });
  for (const label of ["Watching", "Discover", "Activity", "Library"]) {
    await expect(page.getByRole("link", { name: label, exact: true })).toBeVisible();
  }
  await page.getByRole("link", { name: "Library", exact: true }).click();
  await page.getByRole("link", { name: "Account and settings" }).click();
  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page.getByRole("heading", { name: "Welcome to Visto" })).toBeVisible();
});

test("Given an Upcoming episode, When the user opens its season and selects another season, Then the show list stays open", async ({
  page,
}) => {
  await mockSignedInSession(page);
  await page.route("**/api/v1/shows/100", (route) =>
    fulfillJSON(route, { error: "not saved" }, 404),
  );
  await page.route("**/api/v1/calendar**", (route) =>
    fulfillJSON(route, {
      items: [
        {
          show_id: "tv:100",
          title: "The Example Show",
          episode: {
            id: "tv:100:episode:101",
            show_id: "tv:100",
            season_number: 1,
            episode_number: 1,
            air_date: "2026-09-29",
          },
          episode_name: "The Upcoming Episode",
        },
      ],
      next_cursor: null,
    }),
  );
  await page.route("**/api/v1/discover/shows/100", (route) =>
    fulfillJSON(route, {
      media: {
        id: "tv:100",
        tmdb_id: 100,
        type: "tv",
        title: "The Example Show",
        original_title: "The Example Show",
        overview: "A test show.",
        release_date: "2024-01-01",
        poster_path: "",
        original_language: "en",
      },
      seasons: [1, 2].map((number) => ({
        tmdb_id: number,
        season_number: number,
        name: `Season ${number}`,
      })),
      cast: [],
    }),
  );
  await page.route("**/api/v1/discover/shows/100/seasons/*", (route) => {
    const season = Number(new URL(route.request().url()).pathname.split("/").at(-1));
    return fulfillJSON(route, {
      tmdb_id: season,
      season_number: season,
      name: `Season ${season}`,
      episodes: [
        {
          episode: {
            id: `tv:100:episode:${season}01`,
            show_id: "tv:100",
            season_number: season,
            episode_number: 1,
            air_date: "2026-01-01",
          },
          name: `Season ${season} premiere`,
          watched: false,
        },
      ],
    });
  });
  await page.route("**/api/v1/discover/shows/100/seasons/1/episodes/1", (route) =>
    fulfillJSON(route, { name: "The Upcoming Episode", overview: "Episode overview." }),
  );

  await page.goto("/");
  await page.getByRole("tab", { name: "Upcoming" }).click();
  await expect(page.locator(".calendar-card-open")).toHaveAttribute(
    "href",
    "/shows/100/season/1/episode/1",
  );
  await expect(page.locator(".calendar-card-show")).toHaveAttribute("href", "/media/tv/100");
  await page.getByRole("link", { name: /Open The Example Show, season 1, episode 1/ }).click();
  await expect(page.getByRole("heading", { name: "The Upcoming Episode" })).toBeVisible();
  await expect(page.locator(".detail-hero")).toContainText("S1 E1");
  await expect(page.getByRole("link", { name: /^Next episode/ })).toHaveAttribute(
    "href",
    "/shows/100/season/2/episode/1",
  );
  const swipe = (fromX: number, toX: number) =>
    touchDrag(page, { x: fromX, y: 300 }, { x: toX, y: 300 });
  await swipe(300, 120); // swipe left: next episode
  await expect(page).toHaveURL(/\/shows\/100\/season\/2\/episode\/1/);
  await expect(page.locator(".detail-hero")).toContainText("S2 E1");
  await swipe(120, 300); // swipe right: previous episode
  await expect(page).toHaveURL(/\/shows\/100\/season\/1\/episode\/1/);
  await expect(page.locator(".detail-hero")).toContainText("S1 E1");
  await page.getByRole("link", { name: /^Next episode/ }).click();
  await expect(page).toHaveURL(/\/shows\/100\/season\/2\/episode\/1/);
  await expect(page.getByRole("heading", { name: "Season 2 premiere" })).toBeVisible();
  await expect(page.locator(".detail-hero")).toContainText("S2 E1");
  await page.goBack();
  await expect(page).toHaveURL("http://127.0.0.1:4173/");
  await expect(page.getByRole("heading", { name: "Upcoming episodes" })).toBeVisible();
  await page.getByRole("link", { name: /Open The Example Show, season 1, episode 1/ }).click();
  await expect(page.getByRole("heading", { name: "The Upcoming Episode" })).toBeVisible();
  await expect(page.locator(".detail-hero")).toContainText("S1 E1");
  await page.getByRole("link", { name: "The Example Show" }).click();
  await expect(page.getByRole("heading", { name: "Seasons & episodes" })).toBeVisible();
  await expect(page.getByRole("combobox", { name: "Season" })).toHaveValue("Season 1");
  await page.getByRole("combobox", { name: "Season" }).click();
  await page.getByRole("option", { name: "Season 2" }).click();
  await expect(page.getByRole("link", { name: "1 · Season 2 premiere" })).toBeVisible();
  await expect(page.getByText("Episode overview.")).toHaveCount(0);
  await expect(page).toHaveURL(/season=2/);
  await page.getByRole("button", { name: "Back" }).click();
  await expect(page).toHaveURL(/\/shows\/100\/season\/1\/episode\/1/);
  await expect(page.getByRole("heading", { name: "The Upcoming Episode" })).toBeVisible();
  await expect(page.locator(".detail-hero")).toContainText("S1 E1");
  await page.getByRole("button", { name: "Back" }).click();
  await expect(page).toHaveURL("/");
  await expect(page.getByRole("tab", { name: "Upcoming" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await expect(page.getByRole("heading", { name: "Upcoming episodes" })).toBeVisible();

  await page.goto("/media/tv/100?from=%2F&media=tv%3A100&season=2");
  await expect(page.getByRole("link", { name: "1 · Season 2 premiere" })).toBeVisible();
  await page.getByRole("link", { name: "1 · Season 2 premiere" }).click();
  await expect(page.getByRole("heading", { name: "Season 2 premiere" })).toBeVisible();
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
      await fulfillJSON(route, {
        mode: "personal",
        enabled,
        managed_webhook_enabled: false,
        recent_events: [],
      });
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
  await page.goto("/settings");
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

test("Given a discovered Plex account, When an admin assigns it, Then the mapping is shown", async ({
  page,
}) => {
  await mockSignedInSession(page, true);
  let assigned = false;
  await page.route("**/api/v1/users**", (route) =>
    fulfillJSON(
      route,
      route.request().url().includes("cursor=next")
        ? {
            items: [
              {
                id: "user-2",
                name: "Family Member",
                email: "family@example.com",
                role: "user",
                created_at: "2026-01-01T00:00:00Z",
              },
            ],
            next_cursor: null,
          }
        : {
            items: [
              {
                id: user.id,
                name: "Afonso",
                email: "afonso@example.com",
                role: "admin",
                created_at: user.created_at,
              },
            ],
            next_cursor: "next",
          },
    ),
  );
  await page.route("**/api/v1/admin/plex-sync", (route) =>
    fulfillJSON(route, {
      mode: "managed",
      webhook_enabled: true,
      mappings: assigned ? [{ user_id: "user-2", account_id: "456" }] : [],
      observed_accounts: [
        {
          account_id: "456",
          title: "Family",
          last_seen_at: "2026-09-28T12:00:00Z",
          ...(assigned ? { user_id: "user-2" } : {}),
        },
      ],
    }),
  );
  await page.route("**/api/v1/admin/plex-sync/users/user-2", async (route) => {
    expect(route.request().postDataJSON()).toEqual({ account_id: "456" });
    assigned = true;
    await route.fulfill({ status: 204, body: "" });
  });
  await page.goto("/");
  await page.goto("/settings");
  await page.getByRole("tab", { name: "Admin" }).click();
  await expect(page.getByRole("cell", { name: "Family", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Use ID" }).click();
  await page.getByRole("combobox", { name: "Visto user" }).click();
  await page.getByRole("option", { name: "Family Member" }).click();
  await page.getByRole("button", { name: "Assign" }).click();
  await expect(page.getByRole("button", { name: "Remove" })).toBeVisible();
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
  await page.route(/\/api\/v1\/library(?:\?.*)?$/, async (route) => {
    if (route.request().method() === "POST") {
      savedStatuses.push(route.request().postDataJSON().status);
      await fulfillJSON(route, {}, 201);
      return;
    }
    await fulfillJSON(route, { items: [], next_cursor: null });
  });
  await page.route("**/api/v1/library/**", async (route) => {
    if (route.request().method() === "PATCH") {
      savedStatuses.push(route.request().postDataJSON().status);
      await fulfillJSON(route, {});
      return;
    }
    await fulfillJSON(route, {});
  });
  await page.route("**/api/v1/library/lookup", (route) => fulfillJSON(route, []));
  await page.goto("/");
  await page.getByRole("link", { name: "Discover" }).click();
  await page.getByRole("textbox", { name: "Search TMDB" }).fill("Example");
  await expect(page.getByText("The Example Show").first()).toBeVisible();
  await page.getByRole("button", { name: "Add The Example Show to watching" }).click();
  await expect.poll(() => savedStatuses).toEqual(["watching"]);

  await page.getByRole("button", { name: "Save The Example Show for later" }).click();
  await expect.poll(() => savedStatuses).toEqual(["watching", "watchlist"]);
});

test("Media detail links stay short and load after refresh", async ({ page }) => {
  await mockSignedInSession(page);
  const titles = [
    { tmdb_id: 100, type: "tv", title: "Example Show" },
    { tmdb_id: 200, type: "movie", title: "Example Movie" },
  ] as const;
  const media = titles.map((title) => ({
    ...title,
    original_title: title.title,
    overview: "A description that should not appear in the URL.",
    release_date: "2024-01-01",
    poster_path: "",
    original_language: "en",
  }));
  await page.route("**/api/v1/search**", (route) => fulfillJSON(route, media));
  await page.route("**/api/v1/shows/100", (route) =>
    fulfillJSON(route, { error: "not saved" }, 404),
  );
  await page.route("**/api/v1/movies/200", (route) =>
    fulfillJSON(route, { error: "not saved" }, 404),
  );
  await page.route("**/api/v1/discover/shows/100", (route) =>
    fulfillJSON(route, { media: media[0], seasons: [], cast: [] }),
  );
  await page.route("**/api/v1/discover/movies/200", (route) =>
    fulfillJSON(route, { media: media[1], cast: [] }),
  );
  await page.goto("/discover");
  await page.getByRole("textbox", { name: "Search TMDB" }).fill("Example");
  await expect(page).toHaveURL(/\/discover\?q=Example$/);

  for (const title of titles) {
    await page.getByRole("link", { name: `Open details for ${title.title}` }).click();
    await expect(page).toHaveURL(
      new RegExp(
        `/media/${title.type}/${title.tmdb_id}\\?from=%2Fdiscover%3Fq%3DExample&tab=search$`,
      ),
    );
    await expect(page.getByRole("heading", { name: title.title })).toBeVisible();
    await page.reload();
    await expect(page.getByRole("heading", { name: title.title })).toBeVisible();
    await page.getByRole("button", { name: "Back" }).click();
    await expect(page).toHaveURL(/\/discover\?q=Example$/);
    await expect(page.getByRole("textbox", { name: "Search TMDB" })).toHaveValue("Example");
    await expect(page.getByRole("link", { name: `Open details for ${title.title}` })).toBeVisible();
  }
});

test("Discovery search survives reload and clears back to trending", async ({ page }) => {
  await mockSignedInSession(page);
  await page.route("**/api/v1/trending**", (route) =>
    fulfillJSON(route, {
      tv: [{ tmdb_id: 200, type: "tv", title: "Trending Show" }],
      movies: [],
    }),
  );
  await page.route("**/api/v1/search**", (route) =>
    fulfillJSON(route, [{ tmdb_id: 100, type: "tv", title: "Example Show" }]),
  );
  await page.goto("/watch");
  await page.getByRole("link", { name: "Discover" }).click();
  const search = page.getByRole("textbox", { name: "Search TMDB" });
  await search.fill("Example");
  await expect(page).toHaveURL(/\/discover\?q=Example$/);
  await expect(page.getByRole("link", { name: "Open details for Example Show" })).toBeVisible();

  await page.reload();
  await expect(search).toHaveValue("Example");
  await expect(page.getByRole("link", { name: "Open details for Example Show" })).toBeVisible();

  await search.clear();
  await expect(page).toHaveURL(/\/discover$/);
  await expect(search).toHaveValue("");
  await expect(page.getByRole("heading", { name: "Trending TV shows" })).toBeVisible();
  await expect(page.getByText("Trending Show")).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(/\/watch$/);
});

test("TV details show the production status for saved and unsaved shows", async ({ page }) => {
  await mockSignedInSession(page);
  const statuses = [
    ["Returning Series", "Ongoing"],
    ["Ended", "Ended"],
    ["Canceled", "Canceled"],
    ["Cancelled", "Canceled"],
    ["Planned", "Planned"],
    ["In Production", "In Production"],
    ["Pilot", "Pilot"],
    ["Unknown Status", "Unknown Status"],
    ["", null],
  ] as const;
  const media = (status: string, type: "tv" | "movie" = "tv") => ({
    id: `${type}:100`,
    tmdb_id: 100,
    type,
    title: "Example Show",
    original_title: "Example Show",
    overview: "A test show.",
    release_date: "2024-01-01",
    poster_path: "",
    original_language: "en",
    status,
  });

  for (const [status, label] of statuses) {
    await page.route("**/api/v1/shows/100", (route) =>
      fulfillJSON(route, { error: "not saved" }, 404),
    );
    await page.route("**/api/v1/discover/shows/100", (route) =>
      fulfillJSON(route, { media: media(status), seasons: [], cast: [] }),
    );
    await page.goto("/media/tv/100");
    await expect(page.getByRole("heading", { name: "Example Show" })).toBeVisible();
    if (label)
      await expect(page.locator(".detail-facts").getByText(label, { exact: true })).toBeVisible();
    else await expect(page.locator(".detail-facts > span")).toHaveCount(1);
  }

  await page.route("**/api/v1/shows/100", (route) =>
    fulfillJSON(route, {
      media: media("Returning Series"),
      item: { media_id: "tv:100", status: "watching", rating: null, notifications_enabled: false },
    }),
  );
  await page.goto("/media/tv/100");
  await expect(page.locator(".detail-facts").getByText("Ongoing", { exact: true })).toBeVisible();

  await page.route("**/api/v1/shows/**/episodes", (route) =>
    fulfillJSON(route, {
      items: [
        {
          episode: {
            id: "tv:100:episode:1",
            season_number: 1,
            episode_number: 1,
            air_date: "2024-01-01",
          },
          name: "Pilot episode",
          watched: false,
        },
      ],
      next_cursor: null,
    }),
  );
  await page.route("**/api/v1/seasons/**/episodes", (route) =>
    fulfillJSON(route, {
      items: [
        {
          episode: {
            id: "tv:100:episode:1",
            season_number: 1,
            episode_number: 1,
            air_date: "2024-01-01",
          },
          name: "Pilot episode",
          watched: false,
        },
      ],
      next_cursor: null,
    }),
  );
  await page.route("**/api/v1/discover/shows/100/seasons/1/episodes/1", (route) =>
    fulfillJSON(route, { name: "Pilot episode", overview: "Pilot summary." }),
  );
  await page.goto("/shows/100/season/1/episode/1");
  await expect(page.getByRole("heading", { name: "Pilot episode" })).toBeVisible();
  await expect(page.locator(".detail-hero")).toContainText("S1 E1");
  await expect(page.getByText("Pilot summary.")).toBeVisible();
  await page.reload();
  await expect(page.getByRole("heading", { name: "Pilot episode" })).toBeVisible();
  await page.getByRole("link", { name: "Example Show" }).click();
  await expect(page).toHaveURL(/\/media\/tv\/100\?/);
  await expect
    .poll(() => page.evaluate(() => new URL(location.href).searchParams.has("episode_number")))
    .toBe(false);

  await page.route("**/api/v1/shows/tv%3A100/seasons", (route) =>
    fulfillJSON(route, [{ id: "tv:100:season:2", season_number: 2, episode_count: 4 }]),
  );
  await page.route("**/api/v1/seasons/tv%3A100%3Aseason%3A2/episodes", (route) =>
    fulfillJSON(route, {
      items: [
        {
          episode: {
            id: "tv:100:episode:204",
            season_number: 2,
            episode_number: 4,
            air_date: "2025-01-01",
          },
          name: "Second season episode",
          watched: false,
        },
      ],
      next_cursor: null,
    }),
  );
  await page.route("**/api/v1/discover/shows/100/seasons/2/episodes/4", (route) =>
    fulfillJSON(route, { name: "Second season episode", overview: "Season two summary." }),
  );
  await page.goto("/shows/100/season/2/episode/4");
  await expect(page.getByRole("heading", { name: "Second season episode" })).toBeVisible();
  await expect(page.locator(".detail-hero")).toContainText("S2 E4");

  await page.route("**/api/v1/movies/100", (route) =>
    fulfillJSON(route, { media: media("Ended", "movie"), item: {} }),
  );
  await page.goto("/media/movie/100");
  await expect(page.locator(".detail-facts > span")).toHaveCount(1);
});

test("Detail Back restores the show's scroll position and direct links use a fallback", async ({
  page,
}) => {
  await mockSignedInSession(page);
  const show = {
    id: "tv:100",
    tmdb_id: 100,
    type: "tv",
    title: "Long Show",
    original_title: "Long Show",
    overview: "A long season.",
    release_date: "2024-01-01",
    poster_path: "",
    original_language: "en",
  };
  await page.route("**/api/v1/shows/100", (route) =>
    fulfillJSON(route, {
      media: show,
      item: { media_id: show.id, status: "watching", rating: null },
      cast: [{ id: 7, name: "Actor Seven", character: "Lead" }],
    }),
  );
  await page.route("**/api/v1/people/7", (route) =>
    fulfillJSON(route, { tmdb_id: 7, name: "Actor Seven", biography: "", credits: [] }),
  );
  await page.route("**/api/v1/seasons/**/episodes", (route) =>
    fulfillJSON(route, {
      items: Array.from({ length: 30 }, (_, index) => ({
        episode: {
          id: `tv:100:episode:${index + 1}`,
          season_number: 1,
          episode_number: index + 1,
          air_date: "2024-01-01",
        },
        name: `Episode ${index + 1}`,
        watched: false,
      })),
      next_cursor: null,
    }),
  );

  await page.goto("/media/tv/100");
  const episode = page.locator(".episode-row").nth(19);
  await expect(episode).toBeVisible();
  await episode.scrollIntoViewIfNeeded();
  const scrollY = await page.evaluate(() => window.scrollY);
  expect(scrollY).toBeGreaterThan(0);
  const expectEpisodeScrollRestored = async () => {
    await expect(episode).toBeInViewport();
    await expect
      .poll(() => page.evaluate((previous) => Math.abs(window.scrollY - previous), scrollY))
      .toBeLessThan(200);
  };
  await episode.click();
  await expect(page).toHaveURL(/\/shows\/100\/season\/1\/episode\/20/);
  await expect(page.getByRole("heading", { name: "Episode 20" })).toBeVisible();
  await expect(page.locator(".detail-hero")).toContainText("S1 E20");
  await expect(page.getByText("No description is available.")).toBeVisible();
  await page.getByRole("button", { name: "Back", exact: true }).click();
  await expect(page).toHaveURL(/\/media\/tv\/100$/);
  await expectEpisodeScrollRestored();

  await episode.click();
  await page.goBack();
  await expect(page).toHaveURL(/\/media\/tv\/100$/);
  await expectEpisodeScrollRestored();

  await page.getByRole("link", { name: "View Actor Seven" }).scrollIntoViewIfNeeded();
  const castScrollY = await page.evaluate(() => window.scrollY);
  await page.getByRole("link", { name: "View Actor Seven" }).click();
  await expect(page).toHaveURL(/\/people\/7\?/);
  await page.getByRole("button", { name: "Back", exact: true }).click();
  await expect(page).toHaveURL(/\/media\/tv\/100$/);
  // The show/movie query refetches on every mount (refetchOnMount: "always" in
  // queries.ts, so watched/library status stays fresh), which can still be growing the
  // page when the browser's one-shot native scroll restoration fires. That restoration
  // clamps to the page's current max scroll instead of waiting, so it can permanently
  // undershoot by however tall the still-loading content turns out to be. Allow enough
  // slack for that known, harmless native-browser race instead of a moving target.
  await expect
    .poll(() => page.evaluate((previous) => Math.abs(window.scrollY - previous), castScrollY))
    .toBeLessThan(100);

  await page.goto("/media/tv/100?episode=tv%3A100%3Aepisode%3A20");
  await page.getByRole("button", { name: "Back", exact: true }).click();
  await expect(page).toHaveURL(/\/profile$/);

  await page.goto("/people/7");
  await page.getByRole("button", { name: "Back", exact: true }).click();
  await expect(page).toHaveURL(/\/discover$/);

  await page.goto("/users/other-user");
  await page.getByRole("button", { name: "Back to activity" }).click();
  await expect(page).toHaveURL(/\/feed$/);
});

test("Given a TV show detail, When the user uses compact watch controls, Then show and season actions stay clear", async ({
  page,
}) => {
  await mockSignedInSession(page);
  await page.addInitScript(() => localStorage.setItem("visto-theme", "dark"));
  await page.route("**/api/v1/trending**", (route) => fulfillJSON(route, { tv: [], movies: [] }));
  const show = {
    id: "tv:100",
    tmdb_id: 100,
    type: "tv",
    title: "The Example Show",
    original_title: "The Example Show",
    overview: "A test show.",
    release_date: "2024-01-01",
    poster_path: "",
    original_language: "en",
    status: "Ended",
  };
  const episodes = [1, 2].map((number) => ({
    episode: {
      id: `tv:100:episode:${number}`,
      show_id: "tv:100",
      season_number: 1,
      episode_number: number,
      air_date: "2024-01-01",
    },
    name: `Episode ${number}`,
    watched: false,
  }));
  let savedStatus: "watching" | "watchlist" | "completed" | null = null;
  let allWatched = false;
  let showNotificationsEnabled = true;
  await page.route("**/api/v1/shows/**/progress", (route) =>
    fulfillJSON(route, { is_fully_watched: allWatched, watched_episodes: allWatched ? 2 : 0 }),
  );
  await page.route("**/api/v1/shows/100", (route) =>
    savedStatus
      ? fulfillJSON(route, {
          media: show,
          item: {
            media_id: show.id,
            status: savedStatus,
            rating: null,
            notifications_enabled: showNotificationsEnabled,
          },
        })
      : fulfillJSON(route, { error: "not saved" }, 404),
  );
  await page.route("**/api/v1/library/tv%3A100/notifications", async (route) => {
    showNotificationsEnabled = (route.request().postDataJSON() as { enabled: boolean }).enabled;
    await fulfillJSON(route, {});
  });
  await page.route(/\/api\/v1\/library(?:\?.*)?$/, async (route) => {
    if (route.request().method() === "POST") {
      savedStatus = (route.request().postDataJSON() as { status: "watching" | "watchlist" }).status;
      await fulfillJSON(route, {}, 201);
    } else {
      await fulfillJSON(route, { items: [], next_cursor: null });
    }
  });
  let confirmedCompletion = false;
  await page.route("**/api/v1/library/tv%3A100", async (route) => {
    const body = route.request().postDataJSON() as {
      status: "completed" | "watchlist";
      confirm_all_episodes?: boolean;
    };
    if (body.status === "completed") {
      confirmedCompletion = body.confirm_all_episodes === true;
      allWatched = true;
    }
    savedStatus = body.status;
    await fulfillJSON(route, {
      media_id: show.id,
      status: savedStatus,
      created_episode_ids:
        body.status === "completed" ? episodes.map((entry) => entry.episode.id) : [],
    });
  });
  await page.route("**/api/v1/plays/bulk", async (route) => {
    if (route.request().method() === "DELETE") {
      allWatched = false;
      return fulfillJSON(
        route,
        episodes.map((entry, index) => ({
          id: `play-${index + 1}`,
          user_id: user.id,
          episode_id: entry.episode.id,
          watched_at: "2024-02-01T12:00:00Z",
          source: "web",
        })),
      );
    }
    return fulfillJSON(route, []);
  });
  const restoredPlays: unknown[] = [];
  await page.route("**/api/v1/plays", async (route) => {
    restoredPlays.push(route.request().postDataJSON());
    if (restoredPlays.length === episodes.length) allWatched = true;
    await fulfillJSON(route, { id: `restored-${restoredPlays.length}` }, 201);
  });
  await page.route("**/api/v1/shows/**/episodes", (route) =>
    fulfillJSON(route, {
      items: episodes.map((entry) => ({ ...entry, watched: allWatched })),
      next_cursor: null,
    }),
  );
  await page.route("**/api/v1/seasons/**/episodes", (route) =>
    fulfillJSON(route, {
      items: episodes.map((entry) => ({ ...entry, watched: allWatched })),
      next_cursor: null,
    }),
  );
  await page.route("**/api/v1/discover/shows/100", (route) =>
    fulfillJSON(route, {
      media: show,
      seasons: [{ tmdb_id: 1, season_number: 1, name: "Season 1" }],
      cast: [],
    }),
  );
  await page.route("**/api/v1/discover/shows/100/seasons/1", (route) =>
    fulfillJSON(route, { tmdb_id: 1, season_number: 1, name: "Season 1", episodes }),
  );
  await page.route("**/api/v1/discover/tv/100/related", (route) => fulfillJSON(route, []));
  await page.goto("/media/tv/100");
  await expect(
    page.getByRole("button", { name: "Add The Example Show to Watching" }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Save The Example Show for later" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Mark The Example Show watched" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Mark season 1 watched" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Show actions" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Season actions" })).toHaveCount(0);
  await page.getByRole("button", { name: "Mark The Example Show watched" }).click();
  await expect(page.getByRole("dialog", { name: "Mark The Example Show watched" })).toBeVisible();
  await expect(page.getByRole("switch", { name: "Include season 1" })).toBeChecked();
  await page.getByRole("button", { name: "Cancel" }).click();
  await page.getByRole("button", { name: "Mark season 1 watched" }).click();
  await expect(page.getByRole("dialog", { name: "Mark season watched" })).toBeVisible();
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Add The Example Show to Watching" }).click();
  await expect.poll(() => savedStatus).toBe("watching");
  await expect(page.getByRole("button", { name: "Status: Watching. Change list" })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Alerts: Every episode. Change alerts" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Alerts: Every episode. Change alerts" }).click();
  await page.getByRole("radio", { name: "Turn off alerts" }).click();
  await expect(page.getByRole("button", { name: "Alerts: Off. Change alerts" })).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole("button", { name: "Mark The Example Show watched" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Mark season 1 watched" })).toBeVisible();
  await expect(page.getByRole("checkbox", { name: "Episode 1 watched" })).toBeVisible();
  allWatched = true;
  await page.reload();
  await page.getByRole("button", { name: "More actions for The Example Show" }).click();
  await page.getByRole("menuitem", { name: "Mark The Example Show unwatched" }).click();
  await expect(page.getByRole("dialog", { name: "Mark The Example Show unwatched" })).toBeVisible();
  await page.getByRole("button", { name: "Cancel" }).click();
  await expect(page.getByRole("button", { name: "Mark season 1 unwatched" })).toBeVisible();
  await page.getByRole("button", { name: "Mark season 1 unwatched" }).click();
  await page
    .getByRole("dialog", { name: "Mark season unwatched" })
    .getByRole("button", { name: "Mark unwatched" })
    .click();
  await expect(page.getByText("Episodes marked unwatched.")).toBeVisible();
  await page
    .getByText("Episodes marked unwatched.")
    .locator("..")
    .getByRole("button", { name: "Undo" })
    .click();
  await expect.poll(() => restoredPlays.length).toBe(2);
  expect(restoredPlays).toEqual(
    episodes.map((entry) => ({
      episode_id: entry.episode.id,
      watched_at: "2024-02-01T12:00:00Z",
      source: "web",
    })),
  );
  savedStatus = null;
  allWatched = false;
  await page.reload();
  await page.getByRole("button", { name: "Save The Example Show for later" }).click();
  await expect.poll(() => savedStatus).toBe("watchlist");
  await expect(page.getByRole("button", { name: "Status: Watchlist. Change list" })).toBeVisible();
  await expect(page.getByRole("button", { name: /^Alerts:/ })).toHaveCount(0);
  page.once("dialog", (dialog) => dialog.accept());
  await page.getByRole("button", { name: "Status: Watchlist. Change list" }).click();
  await page.getByRole("menuitem", { name: "Completed" }).click();
  await expect.poll(() => confirmedCompletion).toBe(true);
  await expect(page.getByRole("button", { name: "Status: Completed. Change list" })).toBeVisible();
  await page
    .getByText("Show completed.")
    .locator("..")
    .getByRole("button", { name: "Undo" })
    .click();
  await expect.poll(() => savedStatus).toBe("watchlist");
  await expect(page.getByRole("button", { name: "Status: Watchlist. Change list" })).toBeVisible();
});

test("A full-season alert can be switched back to episode alerts", async ({ page }) => {
  await mockSignedInSession(page);
  const show = {
    id: "tv:100",
    tmdb_id: 100,
    type: "tv",
    title: "The Example Show",
    original_title: "The Example Show",
    overview: "",
    release_date: "2024-01-01",
    poster_path: "",
    original_language: "en",
  };
  let notificationMode: "episode" | "season" = "season";
  let updatedMode: string | undefined;
  await page.route("**/api/v1/shows/100", (route) =>
    fulfillJSON(route, {
      media: show,
      item: {
        media_id: show.id,
        status: "watching",
        rating: null,
        notifications_enabled: notificationMode === "episode",
        season_alerts_enabled: notificationMode === "season",
      },
    }),
  );
  await page.route("**/api/v1/library/tv%3A100/notifications", async (route) => {
    const body = route.request().postDataJSON() as { mode: "episode" | "season" };
    updatedMode = body.mode;
    notificationMode = body.mode;
    await fulfillJSON(route, {});
  });
  await page.route("**/api/v1/shows/**/progress", (route) =>
    fulfillJSON(route, { is_fully_watched: false, watched_episodes: 0 }),
  );
  await page.route("**/api/v1/shows/**/episodes", (route) =>
    fulfillJSON(route, { items: [], next_cursor: null }),
  );
  await page.route("**/api/v1/discover/shows/100", (route) =>
    fulfillJSON(route, {
      media: show,
      seasons: [{ tmdb_id: 1, season_number: 1, name: "Season 1" }],
      cast: [],
    }),
  );
  await page.route("**/api/v1/discover/tv/100/related", (route) => fulfillJSON(route, []));
  await page.goto("/media/tv/100");
  const alerts = (mode: string) =>
    page.getByRole("button", { name: `Alerts: ${mode}. Change alerts` });
  await expect(alerts("Every full season")).toBeVisible();
  await alerts("Every full season").click();
  await page.getByRole("radio", { name: "Every new episode" }).click();
  await expect.poll(() => updatedMode).toBe("episode");
  await expect(alerts("Every episode")).toBeVisible();
});

test("Given a movie in Watchlist, When it is marked watched from search, Then Undo restores Watchlist", async ({
  page,
}) => {
  await mockSignedInSession(page);
  const movie = {
    id: "movie:10",
    tmdb_id: 10,
    type: "movie",
    title: "Example Movie",
    original_title: "Example Movie",
    overview: "A test movie.",
    release_date: "2024-01-01",
    poster_path: "",
    original_language: "en",
  };
  let status = "watchlist";
  let playDeleted = false;
  await page.route("**/api/v1/search**", (route) => fulfillJSON(route, [movie]));
  await page.route(/\/api\/v1\/library(?:\?.*)?$/, (route) => {
    if (route.request().method() === "POST") {
      status = route.request().postDataJSON().status;
      return fulfillJSON(route, { media_id: movie.id, status }, 201);
    }
    return fulfillJSON(route, [
      {
        item: {
          media_id: movie.id,
          status,
          rating: 4,
          notifications_enabled: true,
          updated_at: "2026-09-25T00:00:00Z",
        },
        media: movie,
      },
    ]);
  });
  await page.route("**/api/v1/library/lookup", (route) =>
    fulfillJSON(route, [
      {
        item: {
          media_id: movie.id,
          status,
          rating: 4,
          notifications_enabled: true,
          updated_at: "2026-09-25T00:00:00Z",
        },
        media: movie,
      },
    ]),
  );
  await page.route("**/api/v1/plays", async (route) => {
    if (route.request().method() === "POST") status = "completed";
    await fulfillJSON(route, { id: "play-1" }, 201);
  });
  await page.route("**/api/v1/plays/play-1", async (route) => {
    playDeleted = true;
    status = "";
    await route.fulfill({ status: 204, body: "" });
  });
  await page.route("**/api/v1/library/movie%3A10", async (route) => {
    if (route.request().method() === "PATCH") status = route.request().postDataJSON().status;
    await fulfillJSON(route, {});
  });
  await page.goto("/");
  await page.getByRole("link", { name: "Discover" }).click();
  await page.getByRole("textbox", { name: "Search TMDB" }).fill("Example");
  await expect(page.getByText("In Watchlist")).toBeVisible();
  await page.getByRole("button", { name: "Mark Example Movie watched" }).click();
  await expect(page.getByText("Example Movie marked watched.")).toBeVisible();
  await page.getByRole("button", { name: "Undo" }).click();
  await expect.poll(() => playDeleted).toBe(true);
  await expect.poll(() => status).toBe("watchlist");
  await expect(page.getByText("In Watchlist")).toBeVisible();
});

test("Watching links keep native browser navigation and separate the watched action", async ({
  page,
}) => {
  await mockSignedInSession(page);
  await page.setViewportSize({ width: 390, height: 740 });
  await page.route("**/api/v1/plays", (route) => fulfillJSON(route, { id: "play-1" }, 201));
  await page.goto("/watch");

  const row = page.locator(".watch-row", { hasText: "The Example Show" });
  const show = row.locator("a.watch-row-show");
  const episode = row.locator("a.watch-row-title");
  await expect(show).toHaveAttribute("href", "/media/tv/42");
  await expect(episode).toHaveAttribute("href", "/shows/42/season/1/episode/1");
  expect(
    await show.evaluate((element) => element.getBoundingClientRect().height),
  ).toBeGreaterThanOrEqual(43.9);

  const popupPromise = page.context().waitForEvent("page");
  await episode.click({ modifiers: ["ControlOrMeta"] });
  const popup = await popupPromise;
  await expect(popup).toHaveURL(/\/shows\/42\/season\/1\/episode\/1/);
  await popup.close();
  await expect(page).toHaveURL(/\/watch$/);

  await show.click();
  await expect(page).toHaveURL(/\/media\/tv\/42/);
  await page.goBack();
  await expect(page).toHaveURL(/\/watch$/);
  await episode.click();
  await expect(page).toHaveURL(/\/shows\/42\/season\/1\/episode\/1/);
  await page.goBack();
  await expect(page).toHaveURL(/\/watch$/);

  await row.getByRole("button", { name: /Mark The Example Show.*watched/ }).click();
  await expect(page).toHaveURL(/\/watch$/);
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

test("Given another show is being marked watched, When marking a second show, Then both rows stay active and show loading", async ({
  page,
}) => {
  await mockSignedInSession(page);
  const entries = [
    continueEntry,
    {
      ...continueEntry,
      show_id: "tv:43",
      title: "Another Example Show",
      next_episode: { ...continueEntry.next_episode!, id: "tv:43:episode:101" },
    },
  ];
  let releaseFirst!: () => void;
  let releaseSecond!: () => void;
  let startFirst!: () => void;
  let startSecond!: () => void;
  const firstGate = new Promise<void>((resolve) => (releaseFirst = resolve));
  const secondGate = new Promise<void>((resolve) => (releaseSecond = resolve));
  const firstStarted = new Promise<void>((resolve) => (startFirst = resolve));
  const secondStarted = new Promise<void>((resolve) => (startSecond = resolve));
  await page.route("**/api/v1/continue-watching", (route) => fulfillJSON(route, entries));
  await page.route("**/api/v1/plays", async (route) => {
    const { episode_id } = route.request().postDataJSON() as { episode_id: string };
    if (episode_id === "tv:42:episode:101") {
      startFirst();
      await firstGate;
    } else {
      startSecond();
      await secondGate;
    }
    await fulfillJSON(route, { id: `play-${episode_id}` }, 201);
  });
  await page.goto("/");

  const firstAction = page.getByRole("button", {
    name: "Mark The Example Show season 1, episode 1 watched",
    exact: true,
  });
  const secondAction = page.getByRole("button", {
    name: "Mark Another Example Show season 1, episode 1 watched",
    exact: true,
  });
  await firstAction.click();
  await firstStarted;
  await expect(firstAction).toHaveAttribute("aria-busy", "true");
  await expect(secondAction).toBeEnabled();

  await secondAction.click();
  await secondStarted;
  await expect(firstAction).toHaveAttribute("aria-busy", "true");
  await expect(secondAction).toHaveAttribute("aria-busy", "true");

  releaseFirst();
  releaseSecond();
  await expect(page.locator(".watch-row-complete-indicator")).toHaveCount(2);
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
  let healthRequests = 0;
  await page.route("**/health", (route) =>
    fulfillJSON(route, { status: "ok" }, ++healthRequests === 1 ? 503 : 200),
  );
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

test("Given a transient API failure on a direct show link, When the server is healthy, Then no offline notice appears", async ({
  page,
}) => {
  await mockSignedInSession(page);
  let requests = 0;
  let healthRequests = 0;
  await page.route("**/health", (route) => {
    healthRequests++;
    return fulfillJSON(route, { status: "ok" });
  });
  await page.route("**/api/v1/shows/247718", (route) =>
    fulfillJSON(route, { error: "not saved" }, 404),
  );
  await page.route("**/api/v1/discover/shows/247718", (route) => {
    if (++requests === 1) return route.abort("failed");
    return fulfillJSON(route, {
      media: {
        id: "tv:247718",
        tmdb_id: 247718,
        type: "tv",
        title: "MobLand",
        original_title: "MobLand",
        overview: "A show",
        release_date: "2025-03-30",
        poster_path: "",
        original_language: "en",
      },
      seasons: [],
      cast: [],
    });
  });
  await page.goto("/media/tv/247718?from=%2Fdiscover&title=MobLand&type=tv");
  await expect(page.getByRole("heading", { name: "MobLand" })).toBeVisible();
  await expect.poll(() => healthRequests).toBeGreaterThanOrEqual(1);
  await expect(
    page.getByText(
      "You are offline. Saved information may be out of date, and changes need a connection.",
    ),
  ).toHaveCount(0);
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
    expect(manifest.icons).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ src: "/icon-192.png?v=3", purpose: "any" }),
        expect.objectContaining({ src: "/icon-512.png?v=3", purpose: "any" }),
        expect.objectContaining({ src: "/maskable-icon-512.png?v=3", purpose: "maskable" }),
      ]),
    );
    for (const asset of [
      "/icon.svg?v=3",
      "/icon-192.png?v=3",
      "/icon-512.png?v=3",
      "/maskable-icon-512.png?v=3",
      "/apple-touch-icon.png?v=3",
    ]) {
      expect((await page.request.get(asset)).ok(), `${asset} is served`).toBeTruthy();
    }
    await expect(page.locator('link[rel="apple-touch-icon"]')).toHaveAttribute(
      "href",
      "/apple-touch-icon.png?v=3",
    );
  } finally {
    await context.close();
  }
});

test("Given another feed page, When the sentinel enters view, Then it loads without a click", async ({
  page,
}) => {
  await mockSignedInSession(page);
  const requested: string[] = [];
  await page.route(/\/api\/v1\/feed(?:\?.*)?$/, async (route) => {
    const cursor = new URL(route.request().url()).searchParams.get("cursor");
    requested.push(cursor ?? "first");
    await fulfillJSON(route, {
      items: [
        {
          id: cursor ? "event-2" : "event-1",
          display_name: "Afonso",
          kind: "watch",
          title: cursor ? "Second title" : "First title",
          occurred_at: "2026-09-26T12:00:00Z",
        },
      ],
      next_cursor: cursor ? null : "next-page",
    });
  });
  await page.goto("/");
  await page.getByRole("link", { name: "Activity", exact: true }).click();
  await page.getByRole("tab", { name: "Community" }).click();
  await expect(page.getByText("Second title")).toBeVisible();
  expect(requested).toContain("next-page");
});

test("Activity media and user destinations are native links", async ({ page }) => {
  await mockSignedInSession(page);
  await page.route(/\/api\/v1\/feed(?:\?.*)?$/, (route) =>
    fulfillJSON(route, {
      items: [
        {
          id: "event-1",
          user_id: "user-2",
          display_name: "Another viewer",
          kind: "watch",
          title: "Silo",
          media_type: "tv",
          tmdb_id: 100,
          season_number: 2,
          episode_number: 6,
          occurred_at: "2026-09-26T12:00:00Z",
        },
      ],
      next_cursor: null,
    }),
  );
  await page.goto("/feed");
  await page.getByRole("tab", { name: "Community" }).click();
  const row = page.locator(".activity-row", { hasText: "Silo" });
  await expect(row.locator("a.activity-row-open")).toHaveAttribute(
    "href",
    "/shows/100/season/2/episode/6",
  );
  await expect(row.locator("a.activity-show-link")).toHaveAttribute("href", "/media/tv/100");
  await expect(row.locator("a.activity-actor-button")).toHaveAttribute("href", "/users/user-2");
});

test("Feed cards stay compact and touch does not leave a hover border", async ({
  browser,
  page,
}) => {
  const items = [
    {
      id: "episode",
      display_name: "Afonso",
      kind: "watch",
      title: "Silo",
      media_type: "tv",
      artwork_path: "/episode.svg",
      season_number: 3,
      episode_number: 2,
      episode_name: "It's All Good",
      occurred_at: "2026-09-26T12:00:00Z",
    },
    {
      id: "poster",
      display_name: "Afonso",
      kind: "bulk_watch",
      count: 12,
      title: "American Horror Story",
      media_type: "tv",
      artwork_path: "/poster.svg",
      occurred_at: "2026-09-26T12:00:00Z",
    },
  ];
  const setupFeed = async (target: Page) => {
    await mockSignedInSession(target);
    await target.route(/\/api\/v1\/feed(?:\?.*)?$/, (route) =>
      fulfillJSON(route, { items, next_cursor: null }),
    );
    await target.route("https://image.tmdb.org/t/p/**", (route) =>
      route.fulfill({
        contentType: "image/svg+xml",
        body: '<svg xmlns="http://www.w3.org/2000/svg" width="200" height="300"/>',
      }),
    );
    await target.goto("/");
    await target.getByRole("link", { name: "Activity", exact: true }).click();
    await target.getByRole("tab", { name: "Community" }).click();
    await expect(target.locator(".activity-list-feed .activity-row")).toHaveCount(2);
  };

  const mobileContext = await browser.newContext({
    viewport: { width: 390, height: 844 },
    hasTouch: true,
    isMobile: true,
  });
  try {
    const mobilePage = await mobileContext.newPage();
    await setupFeed(mobilePage);
    const mobileRows = mobilePage.locator(".activity-list-feed .activity-row");
    for (const row of await mobileRows.all()) {
      await expect(row).toHaveCSS("height", "128px");
      const layout = await row.evaluate((element) => {
        const art = element.querySelector(".activity-row-art")!.getBoundingClientRect();
        const image = element.querySelector(".activity-row-art img")!.getBoundingClientRect();
        const main = element.querySelector(".activity-row-main")!;
        return {
          artHeight: art.height,
          imageHeight: image.height,
          contentFits: main.scrollHeight <= main.clientHeight,
        };
      });
      expect(layout.imageHeight).toBe(layout.artHeight);
      expect(layout.contentFits).toBe(true);
    }
    const borderBefore = await mobileRows
      .nth(1)
      .evaluate((element) => getComputedStyle(element).borderColor);
    await mobileRows.nth(1).tap();
    await expect(mobileRows.nth(1)).toHaveCSS("border-color", borderBefore);

    await setupFeed(page);
    const desktopRows = page.locator(".activity-list-feed .activity-row");
    for (const row of await desktopRows.all()) {
      await expect(row).toHaveCSS("height", "136px");
    }
    const desktopBorder = await desktopRows
      .nth(1)
      .evaluate((element) => getComputedStyle(element).borderColor);
    await desktopRows.nth(1).hover();
    await expect
      .poll(() => desktopRows.nth(1).evaluate((element) => getComputedStyle(element).borderColor))
      .not.toBe(desktopBorder);
  } finally {
    await mobileContext.close();
  }
});

test("Given another library page, When the user scrolls the list, Then more titles appear", async ({
  page,
}) => {
  await mockSignedInSession(page);
  const entry = (id: number) => ({
    item: {
      media_id: `movie:${id}`,
      status: "watchlist",
      rating: null,
      notifications_enabled: true,
      updated_at: "2026-09-26T12:00:00Z",
    },
    media: {
      id: `movie:${id}`,
      tmdb_id: id,
      type: "movie",
      title: `Library title ${id}`,
      original_title: `Library title ${id}`,
      overview: "",
      release_date: "2026-01-01",
      poster_path: "",
      original_language: "en",
    },
  });
  await page.route(/\/api\/v1\/library(?:\?.*)?$/, async (route) => {
    const url = new URL(route.request().url());
    if (url.searchParams.get("status") !== "watchlist")
      return fulfillJSON(route, { items: [], next_cursor: null });
    if (url.searchParams.has("cursor"))
      return fulfillJSON(route, { items: [entry(2)], next_cursor: null });
    if (url.searchParams.has("limit"))
      return fulfillJSON(route, { items: [entry(1)], next_cursor: "overview-more" });
    return fulfillJSON(route, { items: [entry(1)], next_cursor: "list-more" });
  });
  await page.goto("/");
  await page.getByRole("link", { name: "Library", exact: true }).click();
  await page.getByRole("link", { name: "Show all" }).click();
  await expect(page.getByText("Library title 2")).toBeVisible();
});

test("Expanded library pages filter media before pagination and keep the selection in the URL", async ({
  page,
}) => {
  await mockSignedInSession(page);
  const requested: string[] = [];
  const entry = (type: "movie" | "tv", id: number) => ({
    item: {
      media_id: `${type}:${id}`,
      status: "completed",
      rating: null,
      notifications_enabled: true,
      updated_at: "2026-09-26T12:00:00Z",
    },
    media: {
      id: `${type}:${id}`,
      tmdb_id: id,
      type,
      title: `${type} title ${id}`,
      original_title: `${type} title ${id}`,
      overview: "",
      release_date: "2026-01-01",
      poster_path: "",
      original_language: "en",
    },
  });
  await page.route(/\/api\/v1\/library(?:\?.*)?$/, (route) => {
    const url = new URL(route.request().url());
    requested.push(url.search);
    const type = url.searchParams.get("media_type");
    const status = url.searchParams.get("status");
    if (status !== "completed" && status !== "watchlist")
      return fulfillJSON(route, { items: [], next_cursor: null });
    if (url.searchParams.has("cursor"))
      return fulfillJSON(route, { items: [entry("movie", 2)], next_cursor: null });
    if (type === "tv")
      return fulfillJSON(route, {
        items: [entry("tv", 3)],
        next_cursor: url.searchParams.has("limit") ? "next-page" : null,
      });
    return fulfillJSON(route, { items: [entry("movie", 1)], next_cursor: "next-page" });
  });

  await page.goto("/profile/library/completed");
  await expect(page.getByText("movie title 2")).toBeVisible();
  await page.getByText("TV", { exact: true }).click();
  await expect(page).toHaveURL(/\/profile\/library\/completed\?media_type=tv$/);
  await expect
    .poll(() => page.evaluate(() => localStorage.getItem("visto:library:user-1:filter")))
    .toBe("tv");
  await expect(page.getByText("tv title 3")).toBeVisible();
  await expect(page.getByText("movie title 1")).toHaveCount(0);
  await page.getByText("Movies", { exact: true }).click();
  await expect(page.getByText("movie title 2")).toBeVisible();
  await page.getByRole("combobox", { name: "Sort library media" }).click();
  await page.getByRole("option", { name: "Title" }).click();
  await expect
    .poll(() => page.evaluate(() => localStorage.getItem("visto:library:user-1:sort")))
    .toBe("title");
  expect(
    requested.some((query) => query.includes("media_type=movie") && query.includes("sort=title")),
  ).toBe(true);

  await page.goto("/profile/library/watchlist?media_type=tv");
  await expect(page.getByText("tv title 3")).toBeVisible();
  await page.goto("/profile");
  await page.getByText("TV", { exact: true }).click();
  await page
    .locator(".library-section")
    .filter({ has: page.getByRole("heading", { name: "Completed" }) })
    .getByRole("link", { name: "Show all" })
    .click();
  await expect(page).toHaveURL(/\/profile\/library\/completed\?media_type=tv$/);
});

test("Expanded library search keeps its query and media filter in the URL", async ({ page }) => {
  await mockSignedInSession(page);
  const requests: string[] = [];
  await page.route(/\/api\/v1\/library(?:\?.*)?$/, (route) => {
    const url = new URL(route.request().url());
    requests.push(url.search);
    const title = url.searchParams.get("q") === "alpha" ? "Alpha title" : "Other title";
    return fulfillJSON(route, {
      items: [
        {
          item: {
            media_id: "movie:1",
            status: "completed",
            rating: null,
            notifications_enabled: true,
            updated_at: "2026-09-26T12:00:00Z",
          },
          media: {
            id: "movie:1",
            tmdb_id: 1,
            type: "movie",
            title,
            original_title: title,
            overview: "",
            release_date: "2026-01-01",
            poster_path: "",
            original_language: "en",
          },
        },
      ],
      next_cursor: null,
      total_count: 1,
    });
  });
  await page.goto("/profile/library/completed?media_type=movie");
  const search = page.getByRole("textbox", { name: "Search your library" });
  await search.fill("alpha");
  await expect(page).toHaveURL(/media_type=movie.*q=alpha/);
  await expect(page.getByText("Alpha title")).toBeVisible();
  await page.reload();
  await expect(search).toHaveValue("alpha");
  await search.clear();
  await expect(page).not.toHaveURL(/q=alpha/);
  await expect(page.getByText("Other title")).toBeVisible();
  expect(
    requests.some((query) => query.includes("media_type=movie") && query.includes("q=alpha")),
  ).toBe(true);
});

test("Library previews show full filtered counts in the requested order and restore saved choices", async ({
  page,
}) => {
  await mockSignedInSession(page);
  const statuses = ["watching", "watchlist", "paused", "completed", "dropped"] as const;
  const entry = (status: (typeof statuses)[number], type: "movie" | "tv", id: number) => ({
    item: {
      media_id: `${type}:${id}`,
      status,
      rating: null,
      notifications_enabled: true,
      updated_at: "2026-09-26T12:00:00Z",
    },
    media: {
      id: `${type}:${id}`,
      tmdb_id: id,
      type,
      title: `${status} ${type} ${id}`,
      original_title: `${status} ${type} ${id}`,
      overview: "",
      release_date: "2026-01-01",
      poster_path: "",
      original_language: "en",
    },
  });
  await page.route(/\/api\/v1\/library(?:\?.*)?$/, (route) => {
    const url = new URL(route.request().url());
    const status = url.searchParams.get("status") as (typeof statuses)[number] | null;
    const isTV = url.searchParams.get("media_type") === "tv";
    if (!status) return fulfillJSON(route, { items: [], next_cursor: null, total_count: 0 });
    return fulfillJSON(route, {
      items: [entry(status, isTV ? "tv" : "movie", statuses.indexOf(status) + 1)],
      next_cursor: "more",
      total_count: isTV ? 3 : 12,
    });
  });

  await page.goto("/profile");
  let sections = page.locator(".library-section");
  // Watching, Watchlist and Completed get poster rows; the rest are compact list links.
  await expect(sections).toHaveCount(3);
  await expect(sections.locator("h2")).toHaveText(["Watching", "Watchlist", "Completed"]);
  const moreLists = page.getByRole("navigation", { name: "More lists" });
  await expect(moreLists.getByRole("link")).toHaveText([/Paused\s*12/, /Dropped\s*12/]);
  await expect(sections.first().getByText("12 titles")).toBeVisible();
  await page.getByText("TV", { exact: true }).click();
  await expect(sections.first().getByText("3 titles")).toBeVisible();
  await page.getByRole("combobox", { name: "Sort library media" }).click();
  await page.getByRole("option", { name: "Title" }).click();

  await page.getByRole("link", { name: "Watching", exact: true }).click();
  await page.getByRole("link", { name: "Library", exact: true }).click();
  sections = page.locator(".library-section");
  await expect(sections.first().getByText("3 titles")).toBeVisible();
  await expect(page.getByRole("combobox", { name: "Sort library media" })).toHaveValue("Title");
});

test.describe("touch devices", () => {
  test.use({ hasTouch: true });

  test("Given the To watch tab, When it opens, Then it starts at Up next with recent watches above in the page scroll", async ({
    page,
  }) => {
    await mockSignedInSession(page);
    await page.route("**/api/v1/continue-watching", (route) =>
      fulfillJSON(
        route,
        Array.from({ length: 6 }, (_, i) => ({
          ...continueEntry,
          show_id: `tv:${40 + i}`,
          title: i === 0 ? "The Example Show" : `Another Show ${i}`,
        })),
      ),
    );
    let historyRequests = 0;
    await page.route("**/api/v1/plays?limit=10", (route) => {
      historyRequests += 1;
      return fulfillJSON(route, {
        items: [
          {
            play: {
              id: "p1",
              media_id: null,
              episode_id: "e1",
              watched_at: "2026-09-30T21:50:00Z",
            },
            title: "Silo",
            tmdb_id: 1,
            episode_label: "S02E06",
            episode_name: "Barricades",
          },
          {
            play: {
              id: "p2",
              media_id: "m1",
              episode_id: null,
              watched_at: "2026-09-29T21:50:00Z",
            },
            title: "Dune: Part Two",
            tmdb_id: 2,
          },
        ],
        next_cursor: null,
      });
    });
    await page.setViewportSize({ width: 390, height: 740 });
    await page.goto("/watch");
    const toWatch = page.locator(".watch-row", { hasText: "The Example Show" });
    await expect(toWatch).toBeVisible();
    // The history is fetched ahead of time and always rendered above "Up next".
    await expect.poll(() => historyRequests).toBe(1);
    const section = page.getByLabel("Recently watched");
    const rows = section.locator(".watch-row");
    await expect(rows).toHaveCount(2);
    // Oldest first: the newest play sits right above the episodes still to watch.
    await expect(rows.nth(0)).toContainText("Dune: Part Two");
    await expect(rows.nth(0)).toContainText("Movie");
    await expect(rows.nth(1)).toContainText("Silo");
    await expect(rows.nth(1)).toContainText("S2 E6");
    await expect(rows.nth(1)).toContainText("Barricades");
    await expect(rows.nth(0).locator("a.watch-row-show")).toHaveAttribute("href", "/media/movie/2");
    await expect(rows.nth(0).locator("a.watch-row-title")).toHaveAttribute(
      "href",
      "/media/movie/2",
    );
    await expect(rows.nth(1).locator("a.watch-row-show")).toHaveAttribute("href", "/media/tv/1");
    await expect(rows.nth(1).locator("a.watch-row-title")).toHaveAttribute(
      "href",
      "/shows/1/season/2/episode/6",
    );

    // The page opens at "Up next", with the history just above the viewport.
    await expect.poll(() => page.evaluate(() => window.scrollY)).toBeGreaterThan(0);
    await expect(page.getByRole("heading", { name: "Up next" })).toBeInViewport();
    await expect(toWatch).toBeInViewport();
    await expect(rows.nth(0)).not.toBeInViewport();

    // Scrolling up is ordinary page scroll that goes back in time.
    await page.waitForTimeout(700); // past the short guard against the router's scroll reset
    await page.evaluate(() => window.scrollTo(0, 0));
    await expect(rows.nth(0)).toBeInViewport();
    expect(historyRequests).toBe(1);

    // Choosing Watching from another page lands on Up next again, even while it is still
    // loading and even though the other page left the scroll position elsewhere.
    await page.route("**/api/v1/continue-watching", async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 400));
      await fulfillJSON(
        route,
        Array.from({ length: 6 }, (_, i) => ({
          ...continueEntry,
          show_id: `tv:${40 + i}`,
          title: i === 0 ? "The Example Show" : `Another Show ${i}`,
        })),
      );
    });
    await page.getByRole("link", { name: "Library", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Library" })).toBeVisible();
    await page.evaluate(() => window.scrollTo(0, 120));
    await page.getByRole("link", { name: "Watching", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Up next" })).toBeInViewport();
    await expect(page.locator(".watch-row", { hasText: "The Example Show" })).toBeInViewport();
    await expect(rows.nth(0)).not.toBeInViewport();

    // Tapping Watching again while already there returns to Up next.
    await page.waitForTimeout(700);
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.getByRole("link", { name: "Watching", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Up next" })).toBeInViewport();
  });
});
