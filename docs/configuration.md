# Configuration

Docker Compose reads `.env` for the image tag and every `${...}` value in
`compose.yaml`. You only need to set `VISTO_TMDB_API_KEY` for a standard local
deployment — everything else has a default.

## Environment variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `VISTO_VERSION` | `latest` | Docker image tag used by Compose. Pin to a release such as `0.1.0` for predictable upgrades. |
| `VISTO_TMDB_API_KEY` | empty | TMDB API key. Set this to enable metadata features. |
| `VISTO_GOOGLE_CLIENT_ID` | empty | Google OAuth client ID. Set with the secret and redirect URL to show "Continue with Google" on the sign-in page. |
| `VISTO_GOOGLE_CLIENT_SECRET` | empty | Secret for the Google OAuth client. |
| `VISTO_GOOGLE_REDIRECT_URL` | empty | Exact callback URL registered in Google Cloud, e.g. `https://visto.example.com/api/v1/auth/google/callback`. All three Google variables are required; if any is missing, Google sign-in stays disabled. |
| `VISTO_ALLOW_SIGNUPS` | `true` | Allow public account creation. Set to `false` to disable it. First-admin setup and admin-created accounts remain available either way. |
| `VISTO_PUBLIC_URL` | empty | Public HTTPS origin, e.g. `https://visto.example.com`. Used for browser-origin checks and secure cookies when TLS ends at a proxy. Also required for ChatGPT MCP or Plex webhook URLs. Do not include a path such as `/mcp`. |
| `VISTO_PLEX_SYNC_MODE` | `personal` | `personal` lets users create their own Plex webhook URLs. `managed` lets admins create one shared URL and assign Plex accounts to Visto users. Set it in Compose and restart Visto. |
| `VISTO_TRUSTED_PROXY_CIDRS` | empty | Comma-separated IP ranges for trusted reverse proxies. Set this only when Visto should use forwarded headers to identify visitor IPs and proxy HTTPS. Use the direct proxy address seen by Visto. |
| `VISTO_LISTEN_ADDR` | `:8080` | Address used by the server inside the container. Keep the default with the example port mapping. |
| `VISTO_DATABASE_PATH` | `/data/visto.db` | SQLite database path inside the persistent volume. |
| `VISTO_BACKUP_DIR` | `/data/backups` | Directory for automatic SQLite backups. |
| `VISTO_BACKUP_INTERVAL` | `24h` | Time between automatic backups. |
| `VISTO_BACKUP_RETENTION` | `720h` | How long automatic local backups are kept (30 days by default). |
| `VISTO_BACKUP_SCOPE` | `everything` | `everything` keeps the full database; `user_data` excludes TMDB metadata while retaining user records and their media identifiers. Applies to scheduled and manual (`visto backup`) backups. |
| `VISTO_BACKUP_DESTINATION` | `local` | `local`, `s3`, or `both`. `s3`/`both` require the `VISTO_BACKUP_S3_*` variables below. |
| `VISTO_BACKUP_S3_BUCKET` | empty | S3 bucket for backups. |
| `VISTO_BACKUP_S3_REGION` | empty | S3 region for backups. |
| `VISTO_BACKUP_S3_ENDPOINT` | empty | Optional HTTPS endpoint for an S3-compatible provider. Leave unset for AWS S3. |
| `VISTO_BACKUP_S3_ACCESS_KEY_ID` | empty | S3 access key ID. |
| `VISTO_BACKUP_S3_SECRET_ACCESS_KEY` | empty | S3 secret access key. |
| `VISTO_BACKUP_S3_PATH_STYLE` | `false` | Set to `true` for S3-compatible providers that need path-style addressing. |
| `VISTO_BACKUP_S3_PREFIX` | `visto/` | Object key prefix for uploaded backups. |
| `VISTO_BACKUP_S3_MAX_KEEP` | `30` | Number of scheduled S3 backups to retain. Visto deletes only older scheduled backups matching its own filename pattern under the configured prefix; manual backups are untouched. |
| `VISTO_CATALOG_REFRESH_INTERVAL` | `6h` | How often the backend checks tracked TV metadata for refresh. |
| `VISTO_CATALOG_ACTIVE_TTL` | `24h` | Minimum age of metadata for active shows before refresh. |
| `VISTO_CATALOG_FINISHED_TTL` | `720h` | Minimum age of metadata for ended or cancelled shows before refresh (30 days). |
| `VISTO_ACTIVITY_CLEANUP_INTERVAL` | `24h` | How often the backend removes old activity feed events. |
| `VISTO_ACTIVITY_RETENTION` | `8760h` | How long activity feed events are kept (365 days). Does not affect watch history. |
| `VISTO_SECRET_ENCRYPTION_KEY` | empty | Optional base64-encoded 32-byte key for encrypting users' Pushover credentials and Web Push subscriptions. Generate with `openssl rand -base64 32` and keep it safe — losing it makes saved credentials unreadable. |
| `VISTO_PUSHOVER_INTERVAL` | `15m` | How often the backend checks for new-episode alerts (both Pushover and Web Push). Each user configures their own Pushover app token and user key in Profile. |
| `VISTO_OAUTH_CLEANUP_INTERVAL` | `24h` | How often expired OAuth data is cleaned up. |
| `VISTO_WEB_PUSH_PUBLIC_KEY` / `VISTO_WEB_PUSH_PRIVATE_KEY` | empty | VAPID key pair for Web Push. Generate with `go run ./cmd/vapid`. See [notifications.md](notifications.md). |
| `VISTO_WEB_PUSH_SUBJECT` | empty | A `mailto:` address or your public HTTPS origin, sent with Web Push requests. |

Interval values use Go duration syntax, such as `12h` or `30m`.

The container logs each HTTP request with its method, path, status, response
size, and duration. Query strings and Plex webhook secrets are omitted.

## Using a host directory instead of the named volume

The default named volume works with the image's non-root `visto` user. If you
replace it with a host bind mount, the host directory's permissions apply
inside `/data` — make sure it exists and is writable by the container user. On
Linux, run the container as the directory's owner by setting `user` to its
numeric UID and GID:

```yaml
services:
  visto:
    user: "1000:1000" # replace with the directory owner's UID:GID
    volumes:
      - /home/pi/visto:/data
```

Check the directory owner's IDs with `stat -c '%u:%g' /home/pi/visto` — don't
copy `1000:1000` unless those are the IDs on your server. Keep the default
named volume if you don't need a host directory; setting a host-specific
`user` in the default Compose file can make the named volume unwritable.

To build and run the current source checkout instead of a published image:

```sh
docker compose -f compose.yaml -f compose.build.yaml up -d --build
```

## Reverse proxy and Cloudflare

Set `VISTO_PUBLIC_URL` to your public origin for a standard HTTPS deployment.
Visto uses it to validate browser write requests and mark cookies as secure,
even when HTTPS ends at a reverse proxy — so `VISTO_TRUSTED_PROXY_CIDRS` is
optional for normal browser use.

Visto cannot safely discover a visitor IP by trusting arbitrary request
headers, since a client can forge `X-Forwarded-For`, `X-Real-IP`, or
`CF-Connecting-IP`. Configure each proxy to establish a trusted chain. For
Cloudflare → Nginx Proxy Manager → Visto:

1. Configure Nginx to accept Cloudflare's `CF-Connecting-IP` only from
   Cloudflare's published IP ranges, then forward the resulting client address
   to Visto in `X-Forwarded-For`. Cloudflare documents this in its
   [HTTP header reference](https://developers.cloudflare.com/fundamentals/reference/http-headers/)
   and [visitor IP restoration guide](https://developers.cloudflare.com/support/troubleshooting/restoring-visitor-ips/restoring-original-visitor-ips/).
2. If you want Visto to use that address for IP-based rate limits, set
   `VISTO_TRUSTED_PROXY_CIDRS` to the Nginx Proxy Manager address as seen by
   Visto (not Cloudflare's ranges — Nginx is Visto's direct peer). Prefer a
   stable Nginx address and an exact `/32` (or `/128`) over a broad shared
   Docker subnet.

If `VISTO_TRUSTED_PROXY_CIDRS` is empty, Visto ignores forwarded client IP
headers. Login limits use a normalized account email, so one person's failed
logins don't block the whole household behind the same proxy; MCP OAuth
rate-limits use the peer address and are therefore shared by clients behind
that proxy. Set the CIDR if you need per-client IP limits.

For Nginx Proxy Manager on a Docker network, find its address on the shared
network (replace `npm_network` if yours differs):

```sh
docker network inspect npm_network --format '{{range .Containers}}{{println .Name .IPv4Address}}{{end}}'
```

Then set it as a `/32` CIDR in Visto's Compose environment:

```yaml
environment:
  VISTO_TRUSTED_PROXY_CIDRS: "172.20.0.5/32"
```

Make sure the proxy preserves the original `Host` and forwards
`X-Forwarded-Proto: https`. Don't trust all addresses (`0.0.0.0/0`); if the
proxy's address changes, use a stable address or the smallest reserved
subnet. Recreate Visto after changing the setting:

```sh
docker compose up -d --force-recreate visto
```

## Google sign-in

Create a Google OAuth web client and register the redirect URL above as an
authorized redirect URI, then set all three `VISTO_GOOGLE_*` variables in
`.env` — the Google button appears automatically. Google accounts must have a
verified email address. If that email already belongs to a local account,
Visto links the Google sign-in to it and keeps the password login active. New
Google accounts follow `VISTO_ALLOW_SIGNUPS`; the first administrator must
still be created with email and password before Google users can join.

## Backups

**Manual**, while Visto is running — choose a new destination path inside the
data volume:

```sh
docker compose exec visto /usr/local/bin/visto backup /data/visto-backup.db
```

This won't overwrite an existing file, and stores the backup with owner-only
permissions. Copy it out of the Docker volume to keep a copy off the server.

**Scheduled**: Visto also creates an online SQLite backup every 24 hours in
`/data/backups` and removes its own backups after 30 days by default. Change
the directory, interval, or retention with `VISTO_BACKUP_DIR`,
`VISTO_BACKUP_INTERVAL`, or `VISTO_BACKUP_RETENTION`. Set
`VISTO_BACKUP_SCOPE=user_data` to omit TMDB metadata from scheduled backups —
media and episode identifiers are kept so user records can be restored, but
TMDB details must be re-fetched after restore. `visto backup` (manual) always
makes a full copy regardless of this setting.

**S3**: set `VISTO_BACKUP_DESTINATION` to `s3` or `both` to also (or instead)
upload scheduled backups, along with `VISTO_BACKUP_S3_BUCKET`,
`VISTO_BACKUP_S3_REGION`, `VISTO_BACKUP_S3_ACCESS_KEY_ID`, and
`VISTO_BACKUP_S3_SECRET_ACCESS_KEY`. Set `VISTO_BACKUP_S3_ENDPOINT`
(HTTPS only) and `VISTO_BACKUP_S3_PATH_STYLE=true` for providers that need
them. All of these are read once at startup — restart Visto after changing
them.

**Restore**: stop Visto first and keep a copy of the current database. Copy
the selected backup (downloading it from S3 first, if needed) to the
configured `VISTO_DATABASE_PATH` (normally `/data/visto.db` in the volume),
then start Visto and confirm the expected accounts and library appear before
discarding the pre-restore copy.

## Catalog refresh & activity retention

Tracked TV catalogs refresh in the background, independently of page views —
every 6 hours by default, with a 24-hour freshness window for active shows and
a 30-day window for ended or cancelled shows
(`VISTO_CATALOG_REFRESH_INTERVAL`, `VISTO_CATALOG_ACTIVE_TTL`,
`VISTO_CATALOG_FINISHED_TTL`).

Activity feed events are removed after 365 days by default
(`VISTO_ACTIVITY_RETENTION`, cleaned up every `VISTO_ACTIVITY_CLEANUP_INTERVAL`).
Cleanup runs daily in bounded batches, keyed on the event's recorded creation
time rather than the watch time supplied by the user. It only removes
`activity_events` — watch history in `plays`, library entries, and ratings are
retained.
