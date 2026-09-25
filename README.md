# Visto

Visto is a self-hosted movie and TV tracker. It stores viewing data locally in
SQLite and uses TMDB only for metadata.

## Run with Docker

Install Docker Compose and create a TMDB API key. TMDB access is needed for
search, artwork, cast, episode details, and other metadata features. Copy
`.env.example` to `.env` beside `compose.yaml`, then set `VISTO_TMDB_API_KEY`:

```dotenv
VISTO_TMDB_API_KEY=replace_with_your_tmdb_api_key
VISTO_ALLOW_SIGNUPS=true
# Optional: pin a published Docker release instead of using latest.
# VISTO_VERSION=0.1.0
# Optional: set all three values to enable Google sign-in.
# VISTO_GOOGLE_CLIENT_ID=your-google-oauth-client-id
# VISTO_GOOGLE_CLIENT_SECRET=your-google-oauth-client-secret
# VISTO_GOOGLE_REDIRECT_URL=http://localhost:8080/api/v1/auth/google/callback
```

The repository ignores `.env`. Keep your API key there and do not commit it.
`.env.example` lists all supported environment variables and safe defaults.
Local accounts use a name, email address, and password. Google sign-in is
optional; configure it below to show the Google button on the sign-in screen.

Pull the published image, start Visto, and follow the logs until the server is
ready:

```sh
docker compose pull
docker compose up -d
docker compose logs -f visto
```

Open <http://localhost:8080>. The first account created is the instance
administrator. After setup, that account can use the app and sign in again
normally. Other people can create accounts from the sign-in page when public
signup is enabled. Admins manage accounts under Profile → Admin. The named
Docker volume `visto-data` keeps the SQLite database and backups across
container restarts. Stop the app with `docker compose down`; this keeps the
volume and its data.

By default, Compose uses the `latest` image. Set `VISTO_VERSION` in `.env` to a
release tag such as `0.1.0` to pin an instance. To build and run the current
source checkout instead, use:

```sh
docker compose -f compose.yaml -f compose.build.yaml up -d --build
```

### Published Docker images

Visto publishes multi-platform images for `linux/amd64` and `linux/arm64` to
[`ghcr.io/afonsocraposo/visto`](https://github.com/afonsocraposo/visto/pkgs/container/visto).
Releases are generated from Conventional Commits by Release Please. When
changes are merged to `main`, it opens or updates a release pull request with
the next version and `CHANGELOG.md`. Review and merge that pull request to
create the GitHub release and publish its matching image. Feature commits
(`feat:`) request a minor release, fixes (`fix:`) and performance changes
(`perf:`) request a patch release, and breaking changes request a minor bump
while Visto is below 1.0.0. Documentation, test, build, CI, refactor, and chore
commits do not trigger a release by themselves.

The repository must allow GitHub Actions to create pull requests. GitHub exposes
this as “Allow GitHub Actions to create and approve pull requests” under
Settings → Actions → General. Visto does not use Actions to approve pull
requests; the setting is required only because GitHub does not offer a
separate switch for workflow-created pull requests.

The workflow publishes version and minor-version tags, plus `latest`, after a
release pull request is merged. Images are public and can be pulled without
logging in:

```sh
docker pull ghcr.io/afonsocraposo/visto:0.1.0
```

To pin a deployment to a release, set `VISTO_VERSION=0.1.0` in `.env`. Update
to a newer release with:

```sh
docker compose pull
docker compose up -d
```

### Environment variables

The Compose file below is also the project’s `compose.yaml`. Docker Compose
reads `.env` for the image tag and values shown in `${...}`. Application
settings are passed into the Visto container. You only need to set
`VISTO_TMDB_API_KEY` for a standard local deployment; the other settings have
defaults.

| Variable                         | Default          | Purpose                                                                                                                                                                                                    |
| -------------------------------- | ---------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `VISTO_VERSION`                  | `latest`         | Docker image tag used by Compose. Pin to a release such as `0.1.0` for predictable upgrades.                                                                                                               |
| `VISTO_TMDB_API_KEY`             | empty            | TMDB API key. Set this to enable metadata features.                                                                                                                                                        |
| `VISTO_GOOGLE_CLIENT_ID`         | empty            | Google OAuth client ID. Set with the secret and redirect URL to show “Continue with Google” on the sign-in page.                                                                                           |
| `VISTO_GOOGLE_CLIENT_SECRET`     | empty            | Secret for the Google OAuth client.                                                                                                                                                                        |
| `VISTO_GOOGLE_REDIRECT_URL`      | empty            | Exact callback URL registered in Google Cloud, such as `https://visto.example.com/api/v1/auth/google/callback`. All three Google variables are required; if any is missing, Google sign-in stays disabled. |
| `VISTO_ALLOW_SIGNUPS`            | `true`           | Allow public account creation. Set to `false` to disable it. The first-admin setup and admin-created accounts remain available.                                                                            |
| `VISTO_PUBLIC_URL`               | empty            | Public HTTPS origin, such as `https://visto.example.com`. Set this when ChatGPT MCP or OAuth clients connect, or when users create Plex webhook URLs. Do not include a path such as `/mcp`.                |
| `VISTO_TRUSTED_PROXY_CIDRS`      | empty            | Comma-separated IP ranges for trusted reverse proxies. Set this when deploying behind a proxy; only then are its forwarded client and HTTPS headers trusted.                                               |
| `VISTO_LISTEN_ADDR`              | `:8080`          | Address used by the server inside the container. Keep the default with the example port mapping.                                                                                                           |
| `VISTO_DATABASE_PATH`            | `/data/visto.db` | SQLite database path inside the persistent volume.                                                                                                                                                         |
| `VISTO_BACKUP_DIR`               | `/data/backups`  | Directory for automatic SQLite backups.                                                                                                                                                                    |
| `VISTO_BACKUP_INTERVAL`          | `24h`            | Time between automatic backups.                                                                                                                                                                            |
| `VISTO_BACKUP_RETENTION`         | `720h`           | How long automatic backups are kept (30 days by default).                                                                                                                                                  |
| `VISTO_CATALOG_REFRESH_INTERVAL` | `6h`             | How often the backend checks tracked TV metadata for refresh.                                                                                                                                              |
| `VISTO_CATALOG_ACTIVE_TTL`       | `24h`            | Minimum age of metadata for active shows before refresh.                                                                                                                                                   |
| `VISTO_CATALOG_FINISHED_TTL`     | `720h`           | Minimum age of metadata for ended or cancelled shows before refresh (30 days).                                                                                                                             |
| `VISTO_ACTIVITY_CLEANUP_INTERVAL` | `24h`          | How often the backend removes old activity feed events.                                                                                                                                                    |
| `VISTO_ACTIVITY_RETENTION`       | `8760h`          | How long activity feed events are kept (365 days). This does not remove watch history.                                                                                                                     |
| `VISTO_SECRET_ENCRYPTION_KEY`    | empty            | Optional base64-encoded 32-byte key for encrypting users’ Pushover credentials. Generate it with `openssl rand -base64 32` and keep it safe; losing it makes saved credentials unreadable.                 |
| `VISTO_PUSHOVER_INTERVAL`        | `15m`            | How often the backend checks for new-episode alerts. Each user configures their own Pushover app token and user key in Profile.                                                                            |
| `VISTO_OAUTH_CLEANUP_INTERVAL`   | `24h`            | How often expired OAuth data is cleaned up.                                                                                                                                                                |

The interval values use Go duration syntax, such as `12h` or `30m`. Pushover
and ChatGPT MCP are optional. To use Pushover, set a persistent
`VISTO_SECRET_ENCRYPTION_KEY`; users then enter their own credentials in
Profile. To use MCP or Plex webhooks outside your network, set
`VISTO_PUBLIC_URL` and configure the reverse proxy to pass the required paths
to Visto.

To enable Google sign-in, create a Google OAuth web client and register the
redirect URL shown above as an authorized redirect URI. Set all three Google
variables in `.env`. The Google button appears automatically. Google accounts
must have a verified email address. If that email already belongs to a local
account, Visto links the Google sign-in to that account and keeps its password
login active. New Google accounts follow `VISTO_ALLOW_SIGNUPS`; the first
administrator must still be created with email and password before Google
users can join.

Example `compose.yaml`:

```yaml
services:
  visto:
    image: ghcr.io/afonsocraposo/visto:${VISTO_VERSION:-latest}
    ports:
      - "8080:8080"
    environment:
      VISTO_LISTEN_ADDR: ${VISTO_LISTEN_ADDR:-:8080}
      VISTO_DATABASE_PATH: ${VISTO_DATABASE_PATH:-/data/visto.db}
      VISTO_PUBLIC_URL: ${VISTO_PUBLIC_URL:-}
      VISTO_TRUSTED_PROXY_CIDRS: ${VISTO_TRUSTED_PROXY_CIDRS:-}
      VISTO_ALLOW_SIGNUPS: "${VISTO_ALLOW_SIGNUPS:-true}"
      VISTO_OAUTH_CLEANUP_INTERVAL: ${VISTO_OAUTH_CLEANUP_INTERVAL:-24h}
      VISTO_TMDB_API_KEY: ${VISTO_TMDB_API_KEY:-}
      VISTO_GOOGLE_CLIENT_ID: ${VISTO_GOOGLE_CLIENT_ID:-}
      VISTO_GOOGLE_CLIENT_SECRET: ${VISTO_GOOGLE_CLIENT_SECRET:-}
      VISTO_GOOGLE_REDIRECT_URL: ${VISTO_GOOGLE_REDIRECT_URL:-}
      VISTO_BACKUP_DIR: ${VISTO_BACKUP_DIR:-/data/backups}
      VISTO_BACKUP_INTERVAL: ${VISTO_BACKUP_INTERVAL:-24h}
      VISTO_BACKUP_RETENTION: ${VISTO_BACKUP_RETENTION:-720h}
      VISTO_CATALOG_REFRESH_INTERVAL: ${VISTO_CATALOG_REFRESH_INTERVAL:-6h}
      VISTO_CATALOG_ACTIVE_TTL: ${VISTO_CATALOG_ACTIVE_TTL:-24h}
      VISTO_CATALOG_FINISHED_TTL: ${VISTO_CATALOG_FINISHED_TTL:-720h}
      VISTO_ACTIVITY_CLEANUP_INTERVAL: ${VISTO_ACTIVITY_CLEANUP_INTERVAL:-24h}
      VISTO_ACTIVITY_RETENTION: ${VISTO_ACTIVITY_RETENTION:-8760h}
      VISTO_SECRET_ENCRYPTION_KEY: ${VISTO_SECRET_ENCRYPTION_KEY:-}
      VISTO_PUSHOVER_INTERVAL: ${VISTO_PUSHOVER_INTERVAL:-15m}
    volumes:
      - visto-data:/data
    restart: unless-stopped

volumes:
  visto-data:
```

Set `VISTO_ALLOW_SIGNUPS=false` in the environment or `.env` file to disable
public account creation. This does not disable initial administrator setup or
administrator-created accounts.

To make a consistent manual backup while Visto is running, choose a new
destination path inside the data volume:

```sh
docker compose exec visto /usr/local/bin/visto backup /data/visto-backup.db
```

The command does not overwrite an existing file. It stores the backup with
owner-only permissions. Copy the backup out of the Docker volume to keep a
separate copy away from the server.

To restore a backup, stop Visto first and keep a copy of the current database.
Copy the selected backup to the configured `VISTO_DATABASE_PATH` (normally
`/data/visto.db` in the container volume), then start Visto and open the app to
confirm the expected accounts and library appear. Never overwrite the only copy
of the current database; retain it until the restored instance is verified.

Visto also creates an online SQLite backup every 24 hours in
`/data/backups` and removes its own backups after 30 days. Set
`VISTO_BACKUP_DIR`, `VISTO_BACKUP_INTERVAL`, or `VISTO_BACKUP_RETENTION` to
change the directory, interval, or retention duration (Go duration format,
such as `12h` or `336h` for 14 days).

Tracked TV catalogs refresh in the background, independently of page views.
Defaults are every 6 hours, with a 24-hour freshness window for active shows
and a 30-day freshness window for ended or cancelled shows. These defaults
can be changed with `VISTO_CATALOG_REFRESH_INTERVAL`,
`VISTO_CATALOG_ACTIVE_TTL`, and `VISTO_CATALOG_FINISHED_TTL`.

Activity feed events are removed after 365 days by default. The backend runs
this cleanup daily in bounded batches and uses the event's recorded creation
time, not the watch time supplied by the user. It removes only `activity_events`;
watch history in `plays`, library entries, and ratings are retained. Set
`VISTO_ACTIVITY_RETENTION` or `VISTO_ACTIVITY_CLEANUP_INTERVAL` to change the
retention period or cleanup interval.

## Pushover alerts

Pushover alerts are optional and configured per user. Each user adds their own
Pushover application token and user key under Profile settings, then opts in.
Visto encrypts both credentials before it stores them. To enable secure storage,
set a base64-encoded 32-byte `VISTO_SECRET_ENCRYPTION_KEY` in the server
environment. Generate it with `openssl rand -base64 32` and keep a secure copy:
losing it makes saved credentials unreadable. Users must create or use their
own Pushover application and user keys; the server does not need a shared
Pushover application token. After an upgrade, users must opt in again because
the previous server-wide application token is no longer used. The default
check interval is 15 minutes and can be changed with `VISTO_PUSHOVER_INTERVAL`.

Visto alerts for newly aired regular episodes in shows a user is watching.
Specials, paused or dropped shows, disabled show alerts, and episodes already
marked watched are excluded. Delivery is deduplicated per user and episode.

## Plex watched-content sync

Plex sync is configured separately by each Visto user under Profile →
Settings → Plex watch sync. Plex Pass is required. Create a webhook URL in
Visto, copy it when it is shown, then add it in Plex Web under your account's
webhook settings. Visto shows the URL secret only once; rotate it in Profile if
you lose it. Revoking or rotating the URL immediately invalidates the old URL.

Plex must be able to reach Visto over HTTPS. Set `VISTO_PUBLIC_URL` to the
public origin (for example, `https://visto.example.com`) and forward
`/api/v1/webhooks/plex/` to Visto. Each URL is unique to one Visto user. The
server stores only a hash of its secret. Keep the URL private because it grants
Plex permission to record watches for that account.

Visto processes Plex `media.scrobble` events for movies and TV episodes. It
uses TMDB IDs from Plex when available, then falls back to exact title and
year matching. Ambiguous titles and episodes without an exact season/episode
match are skipped and shown in Recent sync activity. Successful events add the
title to that user's library and record the watched movie or episode. Plex
events only sync watches that happen after webhook setup; existing Plex
history is not imported. Visto suppresses repeat events when a play for the
same item was recorded in the last seven days. The latest 100 event outcomes
are retained per user; Profile displays the latest 20.

## ChatGPT MCP connection

Visto's MCP endpoint is `/mcp`. To connect ChatGPT, set `VISTO_PUBLIC_URL` to
the canonical HTTPS origin used to reach the instance, for example
`https://visto.example.com`. OAuth discovery uses this exact origin, so set it
to the external address exposed by the reverse proxy or secure tunnel. Keep
the `/mcp`, `/oauth/`, and `/.well-known/` paths available through that proxy.
The server supports PKCE authorization, separate read and write permissions,
and per-user Visto accounts.

For deployments behind a reverse proxy, set `VISTO_TRUSTED_PROXY_CIDRS` to the
proxy's IP ranges. Visto trusts forwarded client and HTTPS headers only from
those ranges. OAuth codes and old tokens are cleaned in bounded daily batches;
set `VISTO_OAUTH_CLEANUP_INTERVAL` to change that interval.

In ChatGPT web, enable developer mode, create a custom MCP app, and enter
`https://visto.example.com/mcp` as its endpoint. ChatGPT discovers Visto's
OAuth endpoints and asks each user to sign in with their Visto account and
approve access. Visto uses the identity associated with that user's token,
not a `user_id` sent in an MCP tool call. See [OpenAI's MCP app guide](https://help.openai.com/en/articles/12584461-developer-mode-and-mcp-apps-in-chatgpt).

## Development

Requirements: Go 1.25.13+, Node.js 22+, Air, and a TMDB API key for metadata search.

Install Air once:

```sh
go install github.com/air-verse/air@v1.67.4
```

Run the Go API and the Vite frontend in separate terminals. Docker is not
needed for the development loop.

```sh
# terminal 1
air

# terminal 2
cd frontend
npm install
npm run dev
```

Air rebuilds and restarts the Go server when Go, SQL, or environment files
change. It loads `.env.development` first, then `.env`, so values in `.env`
override matching values from `.env.development`. Vite
reloads the frontend when TypeScript or CSS files change. The Vite server
proxies `/api` and `/health` requests to the Go server on port 8080.

Open `http://localhost:5173` during development. Use Docker on port 8080 for
production-like checks only.

### Inspect the development database

To browse the local SQLite database during development, run this in a separate
terminal:

```sh
cd frontend
npm run db:studio
```

[Drizzle Studio](https://orm.drizzle.team/docs/drizzle-kit-studio) opens at
<https://local.drizzle.studio> and connects to the database at
`VISTO_DATABASE_PATH` from the repository's `.env` file. If unset, it uses
`./data/visto.db`. This is a development/debugging tool; it does not run in
the Visto server or Docker image. Use a local database path, not a
container-only path such as `/data/visto.db`.

### Reset a development database after baseline changes

The current development schema uses one baseline migration. It does not upgrade
databases created from an earlier development schema, including the previous
multi-migration history. Stop Visto, then set `VISTO_DATABASE_PATH` to a new,
empty SQLite file (or remove your old development database after making a backup
if you no longer need its data). Start Visto again and it will create the
current schema. This reset is only needed for a database created before the
current baseline; new databases are initialized on first startup.

### Generate a database diagram

After Visto has created and migrated the local database, generate a Mermaid ER
diagram with:

```sh
cd frontend
npm run db:diagram
```

This reads the database at `VISTO_DATABASE_PATH` from the repository's `.env`
file, or `./data/visto.db` when unset. It opens the database read-only and writes
`docs/database-schema.mmd`. Run the command again after schema migrations to
refresh the diagram.

## Checks

```sh
go test ./...
cd frontend && npm test && npm run build
npx playwright install chromium && npm run test:e2e
```

The project uses BDD-style Given/When/Then test names for domain and application
behaviour. See [SPEC.md](SPEC.md) for the v0.1 product and engineering scope.

## License

Visto is source-available under the [PolyForm Noncommercial License 1.0.0](LICENSE).
Personal and family self-hosting is permitted. Commercial use requires a
separate licence from the copyright holder.
