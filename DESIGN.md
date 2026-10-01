# Visto design

How Visto looks and behaves, and the rules to follow when changing the frontend. The source of
truth is the code: tokens live at the top of [`frontend/src/styles.css`](frontend/src/styles.css),
the Mantine theme in [`frontend/src/app/App.tsx`](frontend/src/app/App.tsx), and shared building
blocks in [`frontend/src/components`](frontend/src/components). Update this file when those change.

## Principles

- **Artwork leads.** Backdrops, stills and posters carry the personality. Layout, scale and
  spacing do the rest. There is no custom typeface and no decorative chrome.
- **One product, not five screens.** Watching, Discover, Activity, Library and details share the
  same tokens, cards, tabs, empty states, errors and motion.
- **Mobile first, desktop composed.** Every screen must work at 320px. Desktop gets more room
  and the cinematic hero, not a different product.
- **Hierarchy before decoration.** What the user acts on comes first: the episode before the
  show badge, the primary action above the fold, personal state in one predictable place.
- **Motion explains, it doesn't perform.** Animate to confirm an action or to show where
  something came from or went. Motion should be felt more than noticed.

Stack: React, Mantine, TanStack Router/Query. Do not add another visual framework.

## Information architecture

| Destination | Route       | Purpose                                                       |
| ----------- | ----------- | ------------------------------------------------------------- |
| Watching    | `/watch`    | Recent watches above, then _Up next_; _Upcoming_ calendar tab |
| Discover    | `/discover` | Search (primary) and trending                                 |
| Activity    | `/feed`     | Own history grouped by day; community feed; People directory  |
| Library     | `/profile`  | Watching, Watchlist, Completed rows; Paused/Dropped as links  |

- Settings live at `/settings`, opened from the account avatar (top bar on desktop, Library
  header on mobile). Admin is a section inside Settings for administrators.
- Detail pages (`/media/…`, `/shows/…/episode/…`, `/people/…`, `/users/…`) keep the tab they
  were opened from (`?tab=`), resolved in `features/navigation/activeTab.ts`.
- Internal values (`feed`, `library`, `/profile`) stay as they are for compatibility. Only the
  labels changed.

### Navigation

- One definition (`features/navigation/navItems.ts`) drives both bars.
- **Below 900px:** a bottom bar, 64px plus the safe area. The active item gets an amber pill
  behind the icon.
- **900px and up:** a 60px top app bar (brand · four destinations · avatar) replaces the
  bottom bar. The active item gets an amber underline and a neutral text colour, never a filled
  amber background.
- The active indicator transitions in about 160ms. Tab changes never use page transitions.

## Color

Use the semantic tokens, never raw hex values in component styles.

| Token                                  | Meaning                                                       |
| -------------------------------------- | ------------------------------------------------------------- |
| `--visto-accent` `#f2b544`             | Brand and primary interaction (primary buttons, active nav)   |
| `--visto-accent-strong`                | Amber text on light surfaces (kickers, upcoming dates)        |
| `--visto-accent-soft`                  | Quiet amber fills (active nav pill)                           |
| `--visto-success` `#48b7ae`            | Watched, success, progress                                    |
| `--visto-success-soft`                 | Watched/complete backgrounds and flashes                      |
| `--visto-danger`                       | Destructive actions and errors                                |
| `--visto-page-bg`                      | Page background (`#f3f6f8` light, `#10141a` dark)             |
| `--visto-surface`, `-raised`, `-glass` | Cards, raised cards, blurred bars                             |
| `--visto-border`, `-strong`            | Default borders; hover borders (neutral, never teal or amber) |
| `--visto-text-muted`                   | Inactive navigation and secondary text                        |
| `--visto-focus`                        | Focus rings (dark amber in light mode, bright amber on dark)  |
| `--visto-artwork-bg`, `-fallback`      | Behind and instead of missing artwork                         |

Rules:

- **Amber** is reserved for brand and primary actions. Do not use it for hover borders, tabs or
  decoration.
- **Teal** only ever means watched, success or progress. Mantine's `teal` palette is remapped so
  that shade 5 is the brand teal.
- **Neutral** carries hover, navigation and secondary controls. **Red** is destructive or error
  only.
- The hero and sign-in screens are always dark. They set `--visto-focus` to the bright accent
  locally.
- Every surface must work in light and dark. Check both.

## Typography

- System font stack (`--visto-font`): `ui-sans-serif, system-ui, -apple-system,
BlinkMacSystemFont, "Segoe UI", sans-serif`. It is fast and local.
- Headings are weight 750 with tight tracking (`-0.045em` for page titles, down to `-0.065em`
  for hero titles). They are balanced where they wrap.
- Kickers are small uppercase labels with `0.06–0.1em` tracking, used sparingly (section
  context, _Your tracking_, day headings).
- Metadata uses one line of `·`-separated facts (`2022 · Ongoing · Drama`, `S2 E7 · 3 left`).
  The separator trails the previous item, so a wrapped line never starts with a dot.
- Episode codes are always `S2 E7` (`Special 3` for season 0). Use `episodeCode` and
  `episodeLabelCode` from `lib/episodePosition.ts`. Never use `S02E07` or `S02 | E07`.
- Use tabular numbers for counts, codes and dates in lists.

## Shape, spacing and elevation

| Radius token             | Value | Use                                  |
| ------------------------ | ----- | ------------------------------------ |
| `--visto-radius-control` | 8px   | Small controls, thumbnails           |
| `--visto-radius-card`    | 12px  | Cards, rows, posters, tracking block |
| `--visto-radius-surface` | 18px  | Large surfaces (person hero)         |
| `--visto-radius-sheet`   | 20px  | Sheets and modals (Mantine `xl`)     |

- Normal cards have no or minimal shadow (`--visto-shadow-card`). Elevation is for desktop
  hover (`--visto-shadow-hover`), stuck bars (`--visto-shadow-bar`) and hero elements like the
  poster (`--visto-shadow-raised`).
- Page gutter is 16px (12px under 680px). Content width tops out at 1120px.
- Detail heroes are full-bleed. The page sets `overflow-x: clip` so nothing scrolls sideways.

### Breakpoints

| Width     | Behaviour                                                     |
| --------- | ------------------------------------------------------------- |
| < 400px   | 2-column poster grids                                         |
| < 680px   | Mobile layouts: compact hero, stacked actions, 3-column grids |
| 681–920px | 4-column grids                                                |
| ≥ 900px   | Top app bar instead of bottom navigation                      |
| > 920px   | 6-column grids                                                |

## Layout patterns

### Cards with a main link

Rows such as Up next, history, episodes, search results, activity and calendar share one
structure. The main control (a link or button for the title) stretches over the whole card with
an `::after` overlay. Secondary controls (show name, watched check, menu, quick actions) sit
above it with `z-index: 2`. Never nest interactive elements, and never use `Paper` with
`role="link"`/`tabIndex`. Use `DetailLink` for real links to media pages: it has an `href`,
supports opening in a new tab, and a plain click uses in-app navigation.

### Watching

- Recent watches (the latest 10, oldest first) render above the tabs in the normal page scroll.
  A small _↑ Recent watches_ hint marks the boundary.
- The page opens scrolled to _Up next_, but only if it is at the top. A restored or user scroll
  position is left alone. There is no nested scroller and no open/closed state.
- Card hierarchy: show (context, link to the show) → **episode title** (content, opens the
  episode) → `S2 E7 · 3 left` (metadata). The still sits on the left and a separate mark-watched
  button on the right.

### Media details

- **Desktop:** cinematic hero with backdrop, poster, title, facts, overview and actions.
- **Mobile:** about 340px of artwork, no large poster, left-aligned, overview clamped to 3
  lines with _More_. The title and primary action must sit above the fold on a regular iPhone.
- **Hero actions** are at most two labelled buttons plus `⋯`. Touch never depends on
  icon-only buttons with tooltips.
  - Unsaved show: `+ Watching` · `Watchlist` · `⋯`
  - Movie: `Mark watched` · `Watchlist` · `⋯`
  - Saved show: `Mark watched` · `⋯` (only when there is something to put in it)
- Secondary actions (rewatch, history, watch date, bulk unwatch) live in `⋯`.
- Each choice has exactly one home. The list a title is in, and removing it from that list, are
  changed only from the Status row in **Your tracking**, never repeated in the hero.
- **Your tracking**, directly below the hero for saved titles, holds Status, Your rating and
  Alerts as rows that open a menu or sheet.

### Episodes

- The navigator `‹ S2 E7 ›` sits right below the hero, with chevrons at least 44px. Desktop
  also shows the neighbouring codes and titles.
- Season browsing has a sticky toolbar, `Season N ▾` · `Mark season`. It only gains a blurred
  surface, border and shadow once stuck (detected with `useStuck`).
- Episode rows read `4 · Title`, then `S3 E4 · In 3 days`, then one line of overview (two on
  desktop). Watched rows show a teal check and dimmed artwork.
- Future episodes stay fully usable. Release wording is _Today_, _Tomorrow_, _In N days_, or a
  date. With no air date, show nothing; never invent a release state.

### Library

- The header is `Library` plus the account avatar, then `All | TV | Movies` and a compact sort.
- Watching, Watchlist and Completed show up to 6 posters with _Show all_. Paused and Dropped
  appear as compact list links with counts.

### Settings

- One column of `SettingsSection` cards (Profile, Appearance, Notifications, Plex, Your data,
  API tokens, Connected apps, Account), with `SettingsSubsection` for divided parts. Section
  titles are compact (1.2rem), never page-title sized. Sign out lives in Account, at the end.
- From 900px a sticky jump list sits to the left and the content column caps at 720px.
- Lists (tokens, connected apps) are divided rows, never cards nested inside the section card.
- Save buttons stay disabled until something changed. Device-dependent states show one status
  line, the most limiting one first.

### Posters

`MediaPosterCard` has two variants:

- `overlay`: the title sits on the artwork. Use it for dense, cinematic grids such as Library
  and related titles.
- `caption`: the title sits below the poster. Use it for legibility in Discover trending and
  filmographies.

Posters always reserve their 2:3 aspect ratio.

## Shared components

| Component                             | Use                                                         |
| ------------------------------------- | ----------------------------------------------------------- |
| `SectionTabs`                         | Every in-page tab set (quiet segmented control, arrow keys) |
| `EmptyState`                          | Icon, title, detail and an action that continues the flow   |
| `QueryError`                          | Any failed load that can be repeated (_Try again_)          |
| `ClampedText`                         | Long copy clamped with _More_/_Less_ only when it overflows |
| `FadeImage`                           | All list and grid artwork (lazy, fade-in)                   |
| `DetailLink`                          | Links to media and episode pages                            |
| `MediaPosterCard`                     | Poster grids (`overlay` / `caption`)                        |
| `RatingStars`                         | Ratings (pop and sequential fill)                           |
| `PosterGridSkeleton`, `ListSkeleton`  | Loading placeholders matching final layout                  |
| `AppLoadingShell`, `PageLoadingShell` | First app load and lazy page loads                          |

Copy conventions:

- Empty states say what will appear and how to get there (_Nothing to watch. Add a show and its
  next released episode will appear here._ followed by _Discover shows_).
- Errors say what failed (_Could not load your library._) and offer _Try again_. Use plain
  alerts only for failed mutations.

## Motion

All durations and easing come from tokens:

- `--visto-motion-fast` (100ms), `--visto-motion-normal` (160ms), `--visto-motion-slow`
  (220ms), `--visto-motion-ambient` (600ms)
- `--visto-ease-standard` (`cubic-bezier(.2,.8,.2,1)`)

| Situation                  | Motion                                                             |
| -------------------------- | ------------------------------------------------------------------ |
| Episode swipe              | Content follows the finger (~0.3×, max 28px, fades to ~0.88)       |
| Previous / next episode    | Directional slide and fade, 20px, ~200ms (swipe and buttons alike) |
| Cancelled swipe            | Snap back, ~140ms                                                  |
| Mark watched (Up next)     | Check pop → success flash → row slides out                         |
| Mark watched (episodes)    | Check pop 0.8→1.1→1, brief success highlight, row stays put        |
| Recent history             | None; native scroll                                                |
| Card tap (touch only)      | Scale 0.98 (posters) / 0.995 (rows), ~90ms                         |
| Navigation                 | Active indicator transition, ~160ms                                |
| Rating                     | Chosen star pops 1→1.18→1; new stars fill ~22ms apart              |
| Watchlist / Watching state | Old state out, new icon or badge scales in, ~170ms                 |
| Tabs                       | Panel opacity 0.85→1 and 3px rise, 120ms; sliding indicator        |
| Library filter / sort      | Previous posters dim to 0.7 while new results load                 |
| Inline editors             | Expand/collapse (grid rows) with fade, ~180ms                      |
| Sticky season toolbar      | Surface, border and shadow transition, ~150ms                      |
| Artwork load               | Opacity 0.6→1, ~150ms                                              |
| Skeleton → content         | Short crossfade, 120ms, no stagger                                 |
| Sign-up name field         | Expand and fade, ~160ms                                            |
| Sign-in backdrop           | Slow crossfade, ~600ms (ambient, not feedback)                     |

Do not:

- Add motion on top of Mantine's Modal, Drawer, Menu, Select, Tooltip or Notifications.
- Use entrance animations per poster, staggered lists, scroll-driven animation, parallax,
  full-route transitions, or skeletons that fly into place.
- Apply press scaling on fine pointers. Desktop uses hover states instead.

### Reduced motion

`prefers-reduced-motion` collapses every motion token to 1ms, sets the press scales to 1 and
cuts keyframe animations short. Episode slide and drag-follow are disabled entirely. State
changes still happen, just immediately. Use the tokens for new motion so this keeps working.

## Accessibility

- Use native elements: links for navigation, buttons for actions, and no interactive
  elements nested inside each other.
- Touch targets are at least 44×44px for navigation, previous/next, mark watched, overflow,
  back, bookmark, calendar and season actions. Icons stay 18–22px; the hit area grows instead,
  using negative margins where layout must not change.
- Every icon-only control has an `aria-label` that names the item (_Mark Silo season 2,
  episode 7 watched_). Tooltips supplement labels and never replace them.
- Focus is always visible through `--visto-focus`. Stretched cards show the ring on the card
  (`:has(:focus-visible)`).
- Tabs follow the tab pattern (`role="tablist"`, `aria-selected`, arrow keys).

## Checklist for UI changes

- [ ] Works at 320px, about 390px (iPhone), tablet and desktop, with no horizontal scroll.
- [ ] Looks right in light and dark.
- [ ] Uses tokens; no new raw colours, radii, shadows or durations.
- [ ] Amber only for primary actions; teal only for watched/success.
- [ ] Loading uses a skeleton matching the final layout; errors offer _Try again_; empty states
      offer a next step.
- [ ] Interactive elements are native, not nested, and at least 44px on touch.
- [ ] Any new motion explains a change or confirms an action, and respects reduced motion.
