# Visto

Visto is a self-hosted movie and TV tracker. It stores viewing data locally in
SQLite and uses TMDB only for metadata.

## Run with Docker

Install Docker Compose and create a TMDB API key. TMDB access is needed for
search, artwork, cast, episode details, and other metadata features. Create a
`.env` file beside `compose.yaml`:

```dotenv
VISTO_TMDB_API_KEY=replace_with_your_tmdb_api_key
VISTO_ALLOW_SIGNUPS=true
```

The repository ignores `.env`. Keep your API key there and do not commit it.

Start Visto and follow the logs until the server is ready:

```sh
docker compose up -d --build
docker compose logs -f visto
```

Open <http://localhost:8080>. The first account created is the instance
administrator. After setup, that account can use the app and sign in again
normally. Other people can create accounts from the sign-in page when public
signup is enabled. Admins manage accounts under Profile → Admin. The named
Docker volume `visto-data` keeps the SQLite database and backups across
container restarts. Stop the app with `docker compose down`; this keeps the
volume and its data.

### Environment variables

The Compose file below is also the project’s `compose.yaml`. Docker Compose
reads `.env` for the values shown in `${...}` and passes them into the Visto
container. You only need to set `VISTO_TMDB_API_KEY` for a standard local
deployment; the other settings have defaults.

| Variable | Default | Purpose |
| --- | --- | --- |
| `VISTO_TMDB_API_KEY` | empty | TMDB API key. Set this to enable metadata features. |
| `VISTO_ALLOW_SIGNUPS` | `true` | Allow public account creation. Set to `false` to disable it. The first-admin setup and admin-created accounts remain available. |
| `VISTO_PUBLIC_URL` | empty | Public HTTPS origin, such as `https://visto.example.com`. Set this when ChatGPT MCP or OAuth clients will connect through a public proxy. Do not include `/mcp`. |
| `VISTO_TRUSTED_PROXY_CIDRS` | empty | Comma-separated IP ranges for trusted reverse proxies. Set this when deploying behind a proxy; only then are its forwarded client and HTTPS headers trusted. |
| `VISTO_LISTEN_ADDR` | `:8080` | Address used by the server inside the container. Keep the default with the example port mapping. |
| `VISTO_DATABASE_PATH` | `/data/visto.db` | SQLite database path inside the persistent volume. |
| `VISTO_BACKUP_DIR` | `/data/backups` | Directory for automatic SQLite backups. |
| `VISTO_BACKUP_INTERVAL` | `24h` | Time between automatic backups. |
| `VISTO_BACKUP_RETENTION` | `720h` | How long automatic backups are kept (30 days by default). |
| `VISTO_CATALOG_REFRESH_INTERVAL` | `6h` | How often the backend checks tracked TV metadata for refresh. |
| `VISTO_CATALOG_ACTIVE_TTL` | `24h` | Minimum age of metadata for active shows before refresh. |
| `VISTO_CATALOG_FINISHED_TTL` | `720h` | Minimum age of metadata for ended or cancelled shows before refresh (30 days). |
| `VISTO_SECRET_ENCRYPTION_KEY` | empty | Optional base64-encoded 32-byte key for encrypting users’ Pushover credentials. Generate it with `openssl rand -base64 32` and keep it safe; losing it makes saved credentials unreadable. |
| `VISTO_PUSHOVER_INTERVAL` | `15m` | How often the backend checks for new-episode alerts. Each user configures their own Pushover app token and user key in Profile. |
| `VISTO_OAUTH_CLEANUP_INTERVAL` | `24h` | How often expired OAuth data is cleaned up. |

The interval values use Go duration syntax, such as `12h` or `30m`. Pushover
and ChatGPT MCP are optional. To use Pushover, set a persistent
`VISTO_SECRET_ENCRYPTION_KEY`; users then enter their own credentials in
Profile. To use MCP from outside your network, set `VISTO_PUBLIC_URL` and
configure the reverse proxy to pass `/mcp`, `/oauth/`, and `/.well-known/` to
Visto.

Example `compose.yaml`:

```yaml
services:
  visto:
    build: .
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
      VISTO_BACKUP_DIR: ${VISTO_BACKUP_DIR:-/data/backups}
      VISTO_BACKUP_INTERVAL: ${VISTO_BACKUP_INTERVAL:-24h}
      VISTO_BACKUP_RETENTION: ${VISTO_BACKUP_RETENTION:-720h}
      VISTO_CATALOG_REFRESH_INTERVAL: ${VISTO_CATALOG_REFRESH_INTERVAL:-6h}
      VISTO_CATALOG_ACTIVE_TTL: ${VISTO_CATALOG_ACTIVE_TTL:-24h}
      VISTO_CATALOG_FINISHED_TTL: ${VISTO_CATALOG_FINISHED_TTL:-720h}
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
go install github.com/air-verse/air@latest
```

Run the Go API and the Vite frontend in separate terminals. Docker is not
needed for the development loop.

```sh
# terminal 1
set -a; source .env; set +a
air

# terminal 2
cd frontend
npm install
npm run dev
```

Air rebuilds and restarts the Go server when Go or SQL files change. Vite
reloads the frontend when TypeScript or CSS files change. The Vite server
proxies `/api` and `/health` requests to the Go server on port 8080.

Open `http://localhost:5173` during development. Use Docker on port 8080 for
production-like checks only.

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
