# Visto product and technical specification

## 1. Product summary

Visto is a lightweight, self-hosted tracker for movies and TV shows. It uses
TMDB for metadata, while each Visto installation owns its users' library,
history, ratings, and settings.

It is designed for a small private installation, such as a family server:

- multiple independent local accounts;
- an installable, mobile-first progressive web application (PWA);
- an API-first backend with REST and OpenAPI;
- a small, opt-in activity feed for people on the same installation; and
- simple deployment with one application container and one SQLite database.

The web app and MCP server are presentation adapters over the same application
use cases. Neither is a privileged shortcut around the domain rules.

## 2. Principles

1. Users own their viewing data.
2. User data is private by default, including from administrators.
3. TMDB is a metadata provider, never the source of user state.
4. Rewatches and corrections are first-class records.
5. TV progress follows the user's furthest watched regular episode, not the
   oldest missing episode.
6. A typical installation requires no Redis, separate worker, or external
   database.

## 3. Licensing

Visto is source-available under the PolyForm Noncommercial License 1.0.0. It
may be self-hosted without charge for permitted noncommercial purposes,
including personal and family use. Commercial hosting, support, resale, and
other commercial use require a separate licence from the copyright holder.

This is intentionally not an OSI-approved open-source licence: commercial-use
restrictions are incompatible with the Open Source Definition. The copyright
holder may operate a paid official hosted service and issue separate commercial
licences. The Visto name and logo should also be protected by a separate
trademark policy.

## 4. Release plan

### v0.1: core tracker

- Local user accounts, roles, sessions, and initial admin setup.
- Local accounts use a name and email address, with password login supported.
- Optional Google OAuth sign-in is enabled only when client ID, client secret,
  and callback URL are configured. Verified Google email addresses link to an
  existing local account with the same email; new Google accounts follow the
  instance signup setting.
- TMDB search and lazy local metadata import for movies and TV shows.
- Per-user library states: `watchlist`, `watching`, `paused`, and `dropped`.
- Movie and episode watch-play history, including rewatches and corrections.
- 1–5 whole-star ratings.
- Derived TV progress, Continue Watching, and an upcoming-episode calendar
  inside the Watch tab.
- Opt-in same-instance activity feed.
- REST API and maintained OpenAPI document.
- CSV and JSON user exports.
- Installable PWA shell, offline messaging, and cached access to recently
  viewed content.
- Docker deployment, SQLite persistence, and manual server backup.

### v0.2: automation and integrations

- Internal scheduled metadata refresh.
- Automated SQLite backups and retention.
- Personal API tokens.
- Optional Pushover notifications.
- Per-user Plex webhook sync for new movie and TV episode scrobbles. Each
  user owns a rotatable secret URL; matching uses TMDB identifiers where
  available and exact title/year/episode matching otherwise. Existing Plex
  history is not imported.

### v0.3: remote MCP

- MCP over Streamable HTTP at the fixed `/mcp` endpoint.
- Support the current stateless MCP protocol and the preceding client protocol
  during migration.
- OAuth authorization server for third-party MCP clients.
- OAuth consent, PKCE, revocation, and scoped access.

## 5. Non-goals for v0.1

- Public profiles, followers, comments, reactions, or direct messages.
- Recommendations and streaming-provider availability.
- Plex, Jellyfin, or automatic scrobbling integrations.
- Shared libraries, household accounts, or shared lists.
- Multiple metadata providers and advanced anime ordering.
- Trakt, Simkl, IMDb, or TMDB-account imports.
- Native mobile applications. The PWA is the mobile client in v0.1.

## 6. Users and privacy

Each user has a separate library, watch history, rating set, token set, and
notification preferences. Metadata is shared by every user on an installation.

Roles are:

- `admin`: manages instance configuration and local accounts.
- `user`: manages only their own data.

An administrator must not automatically see another user's watch history,
ratings, exports, or private activity.

Every user has an activity visibility preference:

- `private` (default): activity does not appear in the instance feed.
- `instance`: eligible activity appears to authenticated users on this Visto
  installation.

### Plex webhook sync

Each user can create, rotate, and revoke one Plex webhook URL under Profile
settings. The URL contains a high-entropy per-user secret. The secret is shown
only when it is issued or rotated; Visto stores only its hash. Creating a URL
requires `VISTO_PUBLIC_URL` and a publicly reachable HTTPS endpoint. Plex Pass
is required on the Plex account.

Visto accepts Plex `media.scrobble` events for movies and TV episodes. It uses
TMDB IDs from Plex when available. Otherwise, a match must have the same media
type and exact normalized title, the same year when Plex supplies one, and a
unique TMDB result. TV episodes also require an exact season and episode
number. Ambiguous or unavailable matches are skipped or reported as failed in
the user's recent sync activity. No raw Plex payload is retained.

The integration is forward-only. A successful event adds its movie or show to
the user's `watching` list and records the watched movie or episode. For TV,
the backend imports show metadata and only the affected season. Plex retries
must not create duplicate plays. A play for the same item in the last seven
days suppresses a new play; a later Plex scrobble is recorded as a rewatch.
Profile shows the latest 20 sync outcomes; the database retains at most 100
events per user. Revoked or rotated URLs stop authenticating immediately.

## 7. Core user workflows

### Library

A user can search TMDB, inspect a title, and add it to their library. Adding a
title requires a status. In search, the primary action for a TV show adds it to
`watching`; a separate `Watch later` action adds it to `watchlist`. Movies use
`Watch later` to add them to `watchlist`. A tracking action, rating, or status
change implicitly creates the user's library relationship when it does not
already exist.

Statuses have these meanings:

- `watchlist`: saved for later.
- `watching`: actively tracking a title.
- `paused`: retain data but hide from Continue Watching and suppress episode
  notifications.
- `dropped`: retain data but hide from Continue Watching and suppress episode
  notifications.

`completed` and `caught_up` are computed display states. They are not stored as
library statuses. A movie is considered watched when it has at least one play.

### Watch plays and corrections

A play is an immutable record that a user watched one movie or one episode at
a point in time. A user may create multiple plays for the same item. The web,
REST, and MCP interfaces must return the play identifier after creation.

To correct history, users delete or edit an individual play by its identifier.
The API must not expose an ambiguous endpoint such as deleting all watches for
an episode.

By default, `watched_at` is the current time. Users can set a past timestamp,
but never a future timestamp. Future episodes may still be marked watched, for
example after viewing a leak; the play is recorded at the current time or an
earlier user-provided time. It counts toward progress immediately, suppresses a
later new-episode notification for that user, and prevents that episode from
being shown as unwatched in the calendar after its air date.

### Ratings

Users can rate library media with whole stars from 1 through 5. `NULL` means
unrated. Ratings are independent per user and belong to the user-media
relationship.

### TV progress and Continue Watching

Progress is derived from episode plays and is never stored as a separate
cursor. The progress point is the highest watched regular episode using the
canonical TMDB aired ordering. It is not the oldest unwatched episode.

Regular episodes are episodes in seasons numbered 1 or higher. Season 0 and
other specials are trackable but do not block normal progress, completion, or
Continue Watching by default.

When a user marks a later regular episode watched and prior regular episodes
are unplayed, Visto asks whether to mark those earlier episodes watched. The
user can choose either:

- mark all identified earlier episodes watched; or
- mark only the selected episode watched.

In both cases the derived progress point advances to the selected episode. The
missing earlier episodes remain historically unplayed when the user selects the
second option.

From a show in their library, a user can browse seasons and episodes and mark
any episode watched. This supports starting partway through a long-running
show; it must not force the user to mark earlier episodes watched.

A show appears in Continue Watching only when all of these are true:

1. The user status is `watching`.
2. The show is not paused or dropped.
3. A released regular episode exists after the user's derived progress point.

If no later released episode exists, the show disappears from Continue
Watching but remains `watching`. It returns automatically when a later episode
becomes available in refreshed metadata. A show with no watched regular
episodes may be shown separately as "Start watching"; it is not a Continue
Watching item.

### Calendar

The calendar lists upcoming episodes from locally cached metadata, grouped in
the user's timezone. It is limited to shows in the user's `watching` state.
Already watched episodes, including episodes watched before their air date, do
not appear as unwatched calendar entries.

### Instance activity feed

The Feed tab is a small, read-only view of recent activity by users whose
visibility is `instance`. It supports only:

- a movie or episode watch;
- a rewatch; and
- a new or changed rating.

Examples:

```text
Margarida watched Severance · S02E05
Afonso rewatched The Matrix
Rui rated Spirited Away · ★★★★★
```

Bulk actions are aggregated into one feed item, for example: "Margarida marked
4 episodes of Severance watched." The feed has cursor pagination. Editing or
deleting a play, or changing visibility to private, removes its eligible feed
activity. There are no comments, reactions, follows, profiles, or activity
from private accounts.

## 8. Metadata

TMDB is the canonical metadata provider for search, title information, seasons,
episodes, images, and air dates. Each Visto instance uses its own TMDB API
credentials. Visto imports metadata lazily when a user opens or adds a result.
It does not mirror the entire TMDB catalogue.

TMDB access is implemented only by the infrastructure adapter using
[`github.com/cyruzin/golang-tmdb`](https://github.com/cyruzin/golang-tmdb).
The adapter initializes a configured client, supplies an HTTP client with an
explicit timeout and bounded connection pool. It does not use the library's
unbounded auto-retry mode; Visto owns the capped, jittered retry and shared
rate-limit policy described below. The rest of Visto depends on a provider
interface, never on the library's client or response types.

The application caches enough raw provider data to support refresh and display,
but it must preserve user data if TMDB changes or removes a record. Media are
unique by `(media_type, tmdb_id)`. All TMDB use must comply with TMDB terms and
required attribution.

TMDB requests are a shared, rate-limited resource within an instance. The
provider adapter must:

- return cached search and metadata results while they are fresh;
- coalesce simultaneous requests for the same resource;
- refresh metadata only when the cache policy requires it;
- bound concurrent TMDB requests and avoid speculative bulk imports;
- on HTTP `429`, honour `Retry-After` when supplied, pause new provider
  requests, and retry with capped exponential backoff and jitter; and
- return a clear, retryable "metadata temporarily unavailable" result to the
  application when retries are exhausted.

No user action, scheduler run, or retry loop may create an unbounded stream of
TMDB requests.

v0.2 refreshes tracked shows more often while airing or upcoming, and less
often after they end. The exact cadence is configuration, not product logic.

## 9. Interface requirements

### Progressive web application

The web client is an installable, mobile-first PWA. It includes a web app
manifest, service worker, application icons, and a standalone display mode.
It must be served over HTTPS in production.

The primary navigation is:

```text
Watch · Search · Feed · Library
```

The Watch tab has `Now` and `Calendar` sub-tabs. `Now` prioritizes Continue
Watching, then upcoming episodes. `Calendar` provides the full upcoming view.
The calendar opens on the current month, starts at today's date in the user's
configured timezone, groups episodes by air date, and supports navigation to
later months. Previous-month navigation is disabled before the current month.
Marking an ordinary next episode watched takes one action. The skipped-prior-
episodes confirmation is the only routine exception.

The Feed tab is positioned between Search and Library. Profile settings are
part of Library, accessed through its account/settings entry point.

The frontend uses React, TypeScript, and Vite, with Mantine for UI components
and TanStack Query (React Query) for server-state fetching, caching, mutation
state, and invalidation. API data must use the shared query cache rather than
duplicated per-screen fetch state. Query keys must include the signed-in user
scope where data can differ by user, and logout must clear private cached data.
Automatic retries are disabled by default; any retry policy must respect
`Retry-After` and avoid retrying non-recoverable responses such as HTTP 429
before the advised time. The interface is simple, clean, responsive, and
accessible. It supports light and dark themes, defaults to the operating
system preference, and lets the user choose a persistent override.

The service worker caches the application shell and recently viewed read-only
responses so users can reopen Visto and view cached content while offline. It
must cache only successful GET responses from an explicit allow-list and keep
at most the 100 most recent API responses per user. Logout clears the active
user cache pointer. The interface clearly shows when cached data may be stale
and offers a connection retry action. v0.1 does not queue or silently retry
offline writes: tracking, ratings, and library changes require a network
connection and show a clear retry action if unavailable.

### REST API

REST is the canonical programmatic interface. Every endpoint is described in
an OpenAPI document from the first release. All user-specific endpoints resolve
the user from the authenticated principal; clients never supply arbitrary user
IDs.

Representative v0.1 endpoints:

```text
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET  /api/v1/search?q=...

GET  /api/v1/library
POST /api/v1/library
PATCH /api/v1/library/{media_id}

GET  /api/v1/movies/{id}
GET  /api/v1/shows/{id}
GET  /api/v1/shows/{id}/progress
GET  /api/v1/shows/{id}/seasons
GET  /api/v1/seasons/{id}/episodes
GET  /api/v1/shows/{id}/episodes

POST   /api/v1/plays
PATCH  /api/v1/plays/{play_id}
DELETE /api/v1/plays/{play_id}

GET /api/v1/continue-watching
GET /api/v1/calendar?from=YYYY-MM-DD&to=YYYY-MM-DD
GET /api/v1/feed?cursor=...

GET   /api/v1/profile/activity-settings
PATCH /api/v1/profile/activity-settings
GET   /api/v1/export/csv
GET   /api/v1/export/json
```

`POST /plays` accepts exactly one of `media_id` or `episode_id`, a
`watched_at` timestamp no later than the current time, and an optional source.
The response includes the play ID and, for a later episode with gaps, the
confirmation information needed by the client before any bulk action.

### MCP

MCP is introduced in v0.3 as a separate presentation adapter. It calls
application use cases directly and must not call the REST API internally.
Remote clients authenticate through OAuth. Tools must use the authenticated
principal rather than accepting `user_id` arguments.

Initial tools include `search_media`, `get_show_progress`,
`get_currently_watching`, `get_upcoming_episodes`, `add_to_watchlist`,
`set_show_status`, `mark_movie_watched`, `mark_episode_watched`, `rate_media`,
and `get_watch_history`.

## 10. Architecture

The backend uses Go, SQLite, REST/OpenAPI, and Docker. The frontend uses React,
TypeScript, Vite, Mantine, and TanStack Query. It remains a client of the same
backend API and application layer.

```text
Presentation (web, REST, MCP)
            ↓
Application (use cases and transactions)
            ↓
Domain (entities, policies, repository interfaces)
            ↑
Infrastructure (SQLite, TMDB, scheduler, Pushover, OAuth)
```

Domain and application code must not depend on HTTP, JSON, SQLite, TMDB, MCP,
or Pushover packages.

Suggested layout:

```text
cmd/server/main.go
internal/domain/
internal/application/
internal/infrastructure/{sqlite,tmdb,backup,oauth,pushover}/
internal/presentation/{http,mcp}/
frontend/src/
  app/                 # application entry and composition
  components/          # small shared UI components
  features/            # feature-owned screens and behavior
    auth/
    feed/
    library/
    navigation/
    search/
    watch/
  types.ts             # shared API and UI types
```

Frontend screens and behavior should stay in their feature folders. Keep the
application entry focused on session/theme setup and composition; do not grow
it into a page or feature implementation file.

## 11. Data model

Core entities are:

```text
User(id, email, name, password_hash?, google_subject?, role, created_at, updated_at)
UserSettings(user_id, timezone, activity_visibility, created_at, updated_at)
PlexWebhook(user_id, token_hash, created_at, last_used_at)
PlexWebhookEvent(id, user_id, fingerprint, status, media summary, occurred_at, created_at)
Media(id, type, tmdb_id, title, original_title, overview, release_date, ...)
Season(id, show_id, tmdb_id, season_number, name, air_date, ...)
Episode(id, show_id, season_id, tmdb_id, season_number, episode_number, ...)
UserMedia(user_id, media_id, status, rating, added_at, updated_at)
Play(id, user_id, media_id?, episode_id?, watched_at, source, created_at)
ActivityEvent(id, user_id, kind, subject_type, subject_id, occurred_at, ...)
```

`Play` requires exactly one of `media_id` and `episode_id`. Database constraints
and application validation enforce this invariant. `ActivityEvent` is a derived
publication record for eligible feed activity; it contains no private data from
users whose visibility is private and is removed or hidden when its source
activity becomes ineligible.

The schema also contains session storage, migrations, and later API-token,
OAuth, notification-delivery, and backup metadata tables. Key indexes include
user-media by `(user_id, status)`, plays by `(user_id, watched_at DESC)` and
`(user_id, episode_id)`, episodes by show ordering and air date, and feed
events by `(occurred_at DESC, id)`.

`plays.source` is validated by application rules but is intentionally open in
SQLite so new integrations do not require rebuilding the growing play-history
table to add a source label. Stable domain invariants, such as valid library
statuses and ratings, remain database constrained.

SQLite initialization enables foreign keys, WAL mode, and a busy timeout:

```sql
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
```

All timestamps are stored as UTC ISO-8601 values.

### SQLite migrations

The database schema is managed through SQL migration files in
`internal/infrastructure/sqlite/migrations/`. While Visto is in development,
this directory contains one baseline migration, `0001_initial_schema.sql`,
which creates the current schema for a fresh database. This baseline replaces
the earlier development migration history. Databases created from an earlier
development schema are not upgraded by it and must be reset before use with
this version.

After the first release, applied migrations are append-only. Schema changes
must add a new migration with a unique, zero-padded version and descriptive
name; they must not edit, rename, reorder, or delete a migration that may have
shipped.

At application startup, the migration runner creates and reads a
`schema_migrations` table containing the migration version, checksum, and apply
time. It applies pending migrations in version order, each inside a transaction,
and records a migration only after its transaction succeeds. Startup fails if a
previously applied migration is missing or its checksum has changed. Tests
verify that a fresh database receives the full current schema, generated IDs
use SQLite integer primary keys, and a second startup is a no-op.

The generated row IDs for users, sessions, plays, activity events, and personal
API tokens use SQLite integer primary keys. The API continues to expose these
IDs as decimal strings. Every `user_id` foreign key uses the matching SQLite
`INTEGER` type; query and API boundaries convert these IDs to strings where
needed.

## 12. Testing

Testing begins with the first feature. The project uses behaviour-driven
development (BDD): product rules are written as executable scenarios using
Given/When/Then language before or alongside implementation.

Each domain rule has focused tests. Application use cases have BDD acceptance
tests against a real SQLite database. HTTP endpoints have authorization and
contract tests derived from OpenAPI. Playwright runs the PWA's critical browser
flows in Chromium, including manifest and service-worker installability,
navigation, light/dark theme choice, and the ordinary one-tap watch action.

Required early scenarios include:

- a later watched episode advances progress without changing missing episodes;
- a paused or dropped show is absent from Continue Watching;
- an unreleased episode marked watched suppresses its later notification;
- private activity never appears in the instance feed;
- TMDB `429` responses pause and retry provider work without multiplying
  requests; and
- one user cannot retrieve another user's data through the API.

## 13. Security

- Hash passwords with Argon2id.
- Use secure, HttpOnly, SameSite session cookies and CSRF protection.
- When `VISTO_PUBLIC_URL` is configured, use its canonical origin for browser
  write checks and mark cookies secure when it uses HTTPS, even if a reverse
  proxy terminates TLS.
- Trust forwarded client-IP and HTTPS headers only from explicitly configured
  `VISTO_TRUSTED_PROXY_CIDRS`. For Cloudflare followed by Nginx, Nginx must
  validate Cloudflare's `CF-Connecting-IP` against Cloudflare source ranges
  and forward the normalized address to Visto.
- Rate-limit login attempts.
- Store personal API tokens, authorization codes, and refresh tokens as hashes.
- Require OAuth PKCE, exact redirect-URI matching, short-lived single-use
  authorization codes, scoped consent, expiry, and revocation.
- Encrypt user Pushover keys at rest when notifications are enabled.
- Authorize every user-scoped repository query using the authenticated
  principal, not only at HTTP-handler level.
- Provide an explicit initial-admin bootstrap mechanism; never ship a default
  administrator password.

## 14. Operations

The preferred deployment is:

```text
docker compose up -d
```

One application container mounts `/data`, which contains the SQLite database
and backups. v0.1 supports an operator-initiated SQLite backup. v0.2 adds a
lightweight in-process scheduler for metadata refresh, duplicate-safe Pushover
delivery, automatic SQLite backups using SQLite's backup API, and retention.

The scheduler runs in the application process and stops on graceful shutdown.
It refreshes tracked TV catalogs every 6 hours by default, checks active or
upcoming shows after 24 hours, and checks ended or cancelled shows after 30
days. Optional Pushover notifications are checked every 15 minutes by default.
The refresh interval and both freshness windows are configurable with
`VISTO_CATALOG_REFRESH_INTERVAL`, `VISTO_CATALOG_ACTIVE_TTL`, and
`VISTO_CATALOG_FINISHED_TTL`. A run processes a bounded batch; a failed TMDB
request is logged and retried at a later scheduled run, not in a tight loop.
Each show refresh fetches its summary and, at most, the latest regular season;
it does not re-fetch every season of long-running shows.

The scheduler removes activity-feed events older than 365 days by default.
Cleanup runs daily in bounded batches and uses each event's recorded creation
time rather than its user-supplied watch time. It deletes only activity events;
play history and library data remain. Operators can configure the policy with
`VISTO_ACTIVITY_RETENTION` and `VISTO_ACTIVITY_CLEANUP_INTERVAL`.

Automatic SQLite backups use the online backup API. They run daily by default,
are written under `/data/backups`, and are retained for 30 days. Operators can
set `VISTO_BACKUP_DIR`, `VISTO_BACKUP_INTERVAL`, and
`VISTO_BACKUP_RETENTION`. Visto creates backups with owner-only file
permissions and only prunes expired backups with its own filename pattern.
The existing manual backup command remains available.

Personal API tokens belong to one user. Their high-entropy secrets are shown
only at creation and stored as hashes. Users can name, optionally expire, list,
and revoke their own tokens. A bearer token has the same user-scoped authority
as that user's web session; it does not bypass authorization or reveal another
user's data.

Pushover is optional and opt-in. An installation provides the Pushover
application token and a secret-encryption key; each user supplies and can
remove their own Pushover user key. User keys are encrypted at rest. Visto may
send one notification for each newly aired regular episode for a show in that
user's `watching` list when notifications are enabled. Paused or dropped
shows, disabled per-show notifications, and episodes the user has already
watched are excluded. Notifications start when a show enters `watching` or
per-show alerts are enabled, so opting in does not send an old-episode backlog.
Notification delivery is deduplicated per user and episode. A failed send is
retried after 15 minutes and then 30 minutes, for at most three attempts. An
uncertain in-progress send is not retried automatically because the provider
may have accepted it before the connection failed.

User exports contain only the authenticated user's data. CSV is a convenient
archive; JSON is designed for reliable future import and round-trip backup.

## 15. Acceptance criteria for v0.1

1. Two users can track the same show without accessing or modifying each
   other's library, history, ratings, or exports.
2. A user can begin with Season 15 while Seasons 1–14 remain unwatched, and
   Visto suggests the episode after their furthest watched episode.
3. A caught-up show leaves Continue Watching and returns when metadata contains
   a later released regular episode.
4. Specials do not block regular-show progress or completion.
5. A user can record, edit, and remove one rewatch without changing other plays.
6. A user can mark an episode watched before its air date; it advances progress
   and suppresses a future notification/calendar prompt for that episode.
7. Ratings accept only whole values from 1 through 5 or no value.
8. Feed users see only opted-in account activity, and bulk marks are aggregated.
9. API operations have OpenAPI definitions and authorization tests.
10. A fresh Docker deployment bootstraps an admin safely and retains data across
    restarts.
11. The PWA is installable, works in light and dark mode, and clearly handles
    an unavailable network connection.
12. TMDB `429` responses are rate-limited and retried safely without an
    uncontrolled request loop.
13. Every implemented domain rule has a corresponding BDD scenario.

## 16. Git workflow

Git is the project record. Each cohesive, verified change is committed with a
Conventional Commit message. Commits include the relevant specification, code,
and tests so the main branch remains usable. The project must not commit
credentials, API keys, generated local databases, backups, or other secrets.

## 17. Future options

Potential later work includes alternate anime orders, imports from Trakt/Simkl/
IMDb, Jellyfin integration, calendar feeds, additional notification
channels, custom lists, tags, statistics, and a yearly review.
