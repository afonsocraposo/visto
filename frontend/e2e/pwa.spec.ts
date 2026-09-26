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
  await page.route(/\/api\/v1\/library(?:\?.*)?$/, (route) => fulfillJSON(route, []));
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
  await page.route(/\/api\/v1\/library(?:\?.*)?$/, async (route) => {
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
  await page.route("**/api/v1/shows/100", (route) => fulfillJSON(route, { error: "not saved" }, 404));
  await page.route("**/api/v1/movies/200", (route) => fulfillJSON(route, { error: "not saved" }, 404));
  await page.route("**/api/v1/discover/shows/100", (route) =>
    fulfillJSON(route, { media: media[0], seasons: [], cast: [] }),
  );
  await page.route("**/api/v1/discover/movies/200", (route) =>
    fulfillJSON(route, { media: media[1], cast: [] }),
  );
  await page.goto("/discover");
  await page.getByRole("textbox", { name: "Search TMDB" }).fill("Example");

  for (const title of titles) {
    await page.getByRole("link", { name: `Open details for ${title.title}` }).click();
    await expect(page).toHaveURL(new RegExp(`/media/${title.type}/${title.tmdb_id}\\?from=%2Fdiscover$`));
    await expect(page.getByRole("heading", { name: title.title })).toBeVisible();
    await page.reload();
    await expect(page.getByRole("heading", { name: title.title })).toBeVisible();
    await page.getByRole("button", { name: "Back" }).click();
    await expect(page).toHaveURL(/\/discover$/);
    await page.getByRole("textbox", { name: "Search TMDB" }).fill("Example");
  }
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
      await expect(
        page.locator(".detail-hero .mantine-Badge-root", { hasText: label }),
      ).toBeVisible();
    else await expect(page.locator(".detail-hero .mantine-Badge-root")).toHaveCount(1);
  }

  await page.route("**/api/v1/shows/100", (route) =>
    fulfillJSON(route, {
      media: media("Returning Series"),
      item: { media_id: "tv:100", status: "watching", rating: null, notifications_enabled: false },
      completed: false,
    }),
  );
  await page.goto("/media/tv/100");
  await expect(
    page.locator(".detail-hero .mantine-Badge-root", { hasText: "Ongoing" }),
  ).toBeVisible();

  await page.route("**/api/v1/shows/**/episodes", (route) =>
    fulfillJSON(route, [
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
    ]),
  );
  await page.goto("/media/tv/100?episode=tv%3A100%3Aepisode%3A1");
  await expect(page.getByRole("heading", { name: "Pilot episode" })).toBeVisible();
  await expect(
    page.locator(".detail-hero .mantine-Badge-root", { hasText: "Ongoing" }),
  ).toBeVisible();

  await page.route("**/api/v1/movies/100", (route) =>
    fulfillJSON(route, { media: media("Ended", "movie"), item: {}, completed: false }),
  );
  await page.goto("/media/movie/100");
  await expect(page.locator(".detail-hero .mantine-Badge-root")).toHaveCount(1);
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
  let savedStatus: "watching" | "watchlist" | null = null;
  let allWatched = false;
  await page.route("**/api/v1/shows/100", (route) =>
    savedStatus
      ? fulfillJSON(route, {
          media: show,
          item: {
            media_id: show.id,
            status: savedStatus,
            rating: null,
            notifications_enabled: true,
          },
          completed: false,
        })
      : fulfillJSON(route, { error: "not saved" }, 404),
  );
  await page.route(/\/api\/v1\/library(?:\?.*)?$/, async (route) => {
    if (route.request().method() === "POST") {
      savedStatus = (route.request().postDataJSON() as { status: "watching" | "watchlist" }).status;
      await fulfillJSON(route, {}, 201);
    } else {
      await fulfillJSON(route, []);
    }
  });
  await page.route("**/api/v1/shows/**/episodes", (route) =>
    fulfillJSON(
      route,
      episodes.map((entry) => ({ ...entry, watched: allWatched })),
    ),
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
  await expect(page.getByRole("combobox", { name: "Current list" })).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole("button", { name: "Mark The Example Show watched" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Mark season 1 watched" })).toBeVisible();
  await expect(page.getByRole("checkbox", { name: "Episode 1 watched" })).toBeVisible();
  allWatched = true;
  await page.reload();
  await expect(page.getByRole("button", { name: "Mark The Example Show unwatched" })).toBeVisible();
  await page.getByRole("button", { name: "Mark The Example Show unwatched" }).click();
  await expect(page.getByRole("dialog", { name: "Mark The Example Show unwatched" })).toBeVisible();
  await page.getByRole("button", { name: "Cancel" }).click();
  await expect(page.getByRole("button", { name: "Mark season 1 unwatched" })).toBeVisible();
  savedStatus = null;
  allWatched = false;
  await page.reload();
  await page.getByRole("button", { name: "Save The Example Show for later" }).click();
  await expect.poll(() => savedStatus).toBe("watchlist");
  await expect(page.getByRole("combobox", { name: "Current list" })).toHaveValue("Watchlist");
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
  await page.route(/\/api\/v1\/library(?:\?.*)?$/, (route) =>
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
        completed: status === "watching",
      },
    ]),
  );
  await page.route("**/api/v1/plays", async (route) => {
    if (route.request().method() === "POST") status = "watching";
    await fulfillJSON(route, { id: "play-1" }, 201);
  });
  await page.route("**/api/v1/plays/play-1", async (route) => {
    playDeleted = true;
    await route.fulfill({ status: 204, body: "" });
  });
  await page.route("**/api/v1/library/movie%3A10", async (route) => {
    if (route.request().method() === "PATCH") status = route.request().postDataJSON().status;
    await fulfillJSON(route, {});
  });
  await page.goto("/");
  await page.getByRole("button", { name: "Discover" }).click();
  await page.getByRole("textbox", { name: "Search TMDB" }).fill("Example");
  await expect(page.getByText("In Watchlist")).toBeVisible();
  await page.getByRole("button", { name: "Mark Example Movie watched" }).click();
  await expect(page.getByText("Example Movie marked watched.")).toBeVisible();
  await page.getByRole("button", { name: "Undo" }).click();
  await expect.poll(() => playDeleted).toBe(true);
  await expect.poll(() => status).toBe("watchlist");
  await expect(page.getByText("In Watchlist")).toBeVisible();
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
    expect(
      manifest.icons.some((icon: { src: string }) => icon.src === "/icon.svg?v=2"),
    ).toBeTruthy();
    expect((await page.request.get("/icon.svg?v=2")).ok()).toBeTruthy();
  } finally {
    await context.close();
  }
});
