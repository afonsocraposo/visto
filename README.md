# Visto

Visto is a self-hosted movie and TV tracker. It stores viewing data locally in
SQLite and uses TMDB only for metadata.

## Run with Docker

Create a TMDB API key, then start Visto:

```sh
export VISTO_TMDB_API_KEY=your_tmdb_api_key
docker compose up -d --build
```

Open `http://localhost:8080`. On first use, create the instance administrator
in the browser. The administrator can add accounts for other people on the
instance. The Docker volume `visto-data` keeps the SQLite database across
restarts.

To make a consistent manual backup while Visto is running, choose a new
destination path inside the data volume:

```sh
docker compose exec visto /usr/local/bin/visto backup /data/visto-backup.db
```

The command does not overwrite an existing file. It stores the backup with
owner-only permissions. Copy the backup out of the Docker volume to keep a
separate copy away from the server.

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

Pushover alerts are optional. Configure `VISTO_PUSHOVER_APP_TOKEN` and a
base64-encoded 32-byte `VISTO_SECRET_ENCRYPTION_KEY` in the server environment.
Generate an encryption key with `openssl rand -base64 32`. Keep a secure copy:
Visto uses it to encrypt each user's Pushover key, and losing it makes saved
keys unreadable. Users can add their Pushover user key and opt in under Profile
settings. The default check interval is 15 minutes and can be changed with
`VISTO_PUSHOVER_INTERVAL`.

Visto alerts for newly aired regular episodes in shows a user is watching.
Specials, paused or dropped shows, disabled show alerts, and episodes already
marked watched are excluded. Delivery is deduplicated per user and episode.

## Development

Requirements: Go 1.22+, Node.js 22+, Air, and a TMDB API key for metadata search.

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
