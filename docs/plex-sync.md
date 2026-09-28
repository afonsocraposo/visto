# Plex watched-content sync

Plex must be able to reach Visto over HTTPS: set `VISTO_PUBLIC_URL` to the
public origin (e.g. `https://visto.example.com`) and forward
`/api/v1/webhooks/plex/` to Visto. Plex Pass is required. The server stores
only a hash of each webhook secret.

Visto processes Plex `media.scrobble` events for movies and TV episodes only.
It uses TMDB IDs from Plex when available, falling back to exact title and
year matching; ambiguous titles and episodes without an exact season/episode
match are skipped and shown in Recent sync activity. A successful match adds
the title to that user's library and records the watch. Only watches that
happen after webhook setup are synced — existing Plex history is not
imported. Repeat events for the same item within seven days are suppressed.
The latest 100 event outcomes are retained per user; Profile shows the latest
20.

There are two sync modes, set with `VISTO_PLEX_SYNC_MODE`.

## Personal mode (default)

Each Visto user configures their own sync under Profile → Settings → Plex
watch sync:

1. Create a webhook URL in Visto — it's shown only once, so copy it
   immediately (rotate it in Profile if you lose it; revoking or rotating
   immediately invalidates the old URL).
2. Add it in Plex Web under your account's webhook settings.
3. To limit sync to your own Plex viewer, enter its numeric account ID
   *before* creating the webhook URL. If you don't know it, leave the field
   empty, create the URL, and watch something on your Plex profile — Visto
   will skip the event but show the observed account ID under Recent sync
   activity. Enter that ID, rotate the URL, and replace the URL in Plex.

An unset account ID skips every event, and existing webhook URLs skip events
until rotated with an account ID. Each webhook URL is unique to one Visto
user — keep it private, since it grants Plex permission to record watches for
that account.

## Managed mode

For a Plex Pass owner whose family members don't have Plex Pass, set
`VISTO_PLEX_SYNC_MODE=managed` and restart Visto:

1. In the Admin panel, create the shared webhook URL and add it to the Plex
   Pass owner's Plex webhook settings.
2. Start playback on each family account. The Admin panel lists recently seen
   Plex account IDs, names, and last-seen times (an account is discovered on
   playback, but only records a watch once Plex sends `media.scrobble`; admins
   can also enter an ID manually). The latest 100 distinct observed accounts
   are retained — full Plex payloads are not stored for discovery.
3. Assign each ID to a Visto user. Only assigned accounts record watches.

Existing personal-mode URLs remain saved but stop syncing in managed mode;
they work again if the mode returns to `personal`.
