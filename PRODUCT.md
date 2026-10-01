# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Self-hosters in general: anyone with a homelab or small server who installs Visto
for themselves and the people they share it with (family, housemates, friends).
Two roles matter:

- **Members** track what they watch: they log episodes and movies right after
  watching, check what comes next, keep a watchlist and rate titles.
- **The instance owner** (the first account, an administrator) deploys Visto with
  Docker, configures it through environment variables, and manages accounts and
  shared integrations. Owners are strangers to the author, so setup and settings
  copy must make sense without outside help.

## Product Purpose

Visto is a self-hosted movie and TV tracker. Each installation owns its users'
library, watch history, ratings and settings in one SQLite database; TMDB only
supplies metadata. Success means people log a watch in a few taps, always know
what to watch next, and never lose or leak their history.

## Positioning

- **Data ownership.** History lives on the owner's server, is private by default
  (even from administrators), and can be exported in full as JSON or CSV. TMDB is
  never the source of user state.
- **Integrations.** Plex webhook sync, Pushover and Web Push alerts for new
  episodes and releases, personal API tokens, and a remote MCP server with OAuth,
  so ChatGPT and other clients can read and update a user's data.
- **Small-group social.** An opt-in activity feed and people directory among
  users of the same instance. There are no public profiles, followers, comments
  or reactions.

## Operating Context

- Mostly used on phones as an installed PWA, in short sessions right after
  watching something. Desktop is secondary: browsing, library management,
  settings and administration.
- Deployed with Docker Compose (one container plus SQLite) on `linux/amd64` or
  `linux/arm64`, often behind a reverse proxy. Optional features depend on
  environment variables (TMDB key, Google sign-in, encryption key for Pushover,
  Web Push, S3 backups).
- People often move over from other trackers. Bingers import exists today.

## Capabilities and Constraints

- Library states: `watchlist`, `watching`, `paused`, `dropped`, `completed`.
  The UI labels the destinations Watching, Discover, Activity and Library.
- Watch history covers movies and episodes, and treats rewatches and corrections
  as real records. Ratings are 1–5 whole stars.
- TV progress follows the user's furthest watched regular episode, not the
  oldest missing one. Up next and the Upcoming calendar derive from that.
- REST API with a maintained OpenAPI document. The web app and MCP server go
  through the same application use cases.
- No Redis, separate worker or external database for a typical install.
- Source-available under PolyForm Noncommercial 1.0.0. Personal and family
  self-hosting is free; commercial use needs a separate licence.
- Out of scope unless decided otherwise: recommendations, streaming
  availability, public social features.

## Brand Commitments

- The product name is **Visto**. The app icon and maskable icons live in
  `frontend/public/`.
- Copy is plain and direct: it says what happened and what to do next, with no
  hype.

## Evidence on Hand

- Docs: `README.md`, `docs/configuration.md`, `docs/notifications.md`,
  `docs/plex-sync.md`, `docs/mcp.md`, `docs/plans/SPEC.md`.
- Artwork comes from TMDB at runtime. There are no testimonials, user counts,
  press or case studies, and future work must not invent them.

## Product Principles

1. **Users own their viewing data.** Private by default, exportable, never held
   hostage by a third party.
2. **Logging a watch should be quick.** The phone flow right after watching comes
   before everything else.
3. **Integrations are optional.** Every integration explains what it needs, and
   the app works fully without any of them.
4. **Social is opt-in and stays inside the instance.**
5. **Strangers can run it.** Every self-hosted owner must be able to install,
   configure and recover Visto from the UI and docs alone.
