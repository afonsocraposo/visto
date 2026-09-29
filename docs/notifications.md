# Notifications

Visto can alert users about new episodes, complete seasons, and movie releases
over Pushover, Web Push, or both. Both channels share the
`VISTO_PUSHOVER_INTERVAL` check interval (default `15m`) and require a
persistent `VISTO_SECRET_ENCRYPTION_KEY` (see
[configuration.md](configuration.md)) so per-user credentials can be stored
encrypted.

## What triggers an alert

For a show marked Watching, its notification button selects alerts for every
new regular episode or each season when all its listed regular episodes have
aired. The two choices are exclusive and apply to future seasons. Specials,
paused or dropped shows, and episodes already marked watched are excluded.
Delivery is deduplicated per user and episode or season. Alerts use the user's
enabled Pushover and Web Push channels. Leaving Watching cancels pending season
alerts.

For an unreleased movie in Watchlist, **Release alert · On/Off** controls a
one-time alert for its release date. Visto refreshes watchlisted unreleased
movie metadata through the existing catalog refresh worker.

## Pushover

Each user adds their own Pushover application token and user key under
Profile settings, then opts in — the server does not need a shared Pushover
application token. **Send test notification** checks saved credentials
without turning on scheduled alerts. After an upgrade, users must opt in again
because the previous server-wide application token is no longer used.

## Web Push

Set the persistent `VISTO_SECRET_ENCRYPTION_KEY`, `VISTO_WEB_PUSH_PUBLIC_KEY`,
`VISTO_WEB_PUSH_PRIVATE_KEY`, and `VISTO_WEB_PUSH_SUBJECT` (a `mailto:`
address or your public HTTPS origin) on the server. Generate the VAPID key
pair with `go run ./cmd/vapid` and keep the private key secret and stable —
changing it invalidates existing device subscriptions.

Visto must be served over HTTPS, with `VISTO_PUBLIC_URL` set to its public
origin. Each user enables notifications per device in Profile and can use
**Send test notification** there. On iPhone and iPad, install Visto to the
Home Screen before enabling notifications.
