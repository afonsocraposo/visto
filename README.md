# Visto

Visto is a self-hosted movie and TV tracker. It stores viewing data locally in
SQLite and uses TMDB only for metadata.

## Quick start (Docker)

1. Install Docker Compose and create a [TMDB API key](https://www.themoviedb.org/settings/api) — needed for search, artwork, cast, episode details, and other metadata.
2. Copy `.env.example` to `.env` beside `compose.yaml` and set `VISTO_TMDB_API_KEY`. `.env.example` lists every supported variable with safe defaults; `.env` is git-ignored, so keep your key there.
3. Start Visto and follow the logs until it's ready:

   ```sh
   docker compose pull
   docker compose up -d
   docker compose logs -f visto
   ```

4. Open <http://localhost:8080>. The first account created becomes the instance administrator; admins manage accounts under Profile → Admin. Public signup can be turned off with `VISTO_ALLOW_SIGNUPS=false`.

The named volume `visto-data` holds the SQLite database and backups and survives `docker compose down`. See [docs/configuration.md](docs/configuration.md) for the full environment variable reference, host-directory volumes, backups & S3, reverse proxies, and Google sign-in.

## Optional features

- **[Pushover & Web Push alerts](docs/notifications.md)** — per-user notifications for new episodes and movie releases.
- **[Plex watched-content sync](docs/plex-sync.md)** — import watches from Plex via webhook.
- **[ChatGPT MCP connection](docs/mcp.md)** — connect Visto to ChatGPT as an MCP app.

## Releases

Multi-platform images (`linux/amd64`, `linux/arm64`) are published to [ghcr.io/afonsocraposo/visto](https://github.com/afonsocraposo/visto/pkgs/container/visto). [Release Please](https://github.com/googleapis/release-please) opens a release PR from Conventional Commits merged to `main`; merging it cuts the GitHub release and publishes the matching image (`feat:` → minor, `fix:`/`perf:` → patch, breaking changes → minor while below 1.0.0; docs/test/build/CI/refactor/chore commits don't trigger a release).

Pin a deployment to a release with `VISTO_VERSION=0.1.0` in `.env`, then `docker compose pull && docker compose up -d` to upgrade.

> The repository needs "Allow GitHub Actions to create and approve pull requests" enabled under Settings → Actions → General, so the release workflow can open its PR.

## Development

Requirements: Go 1.25.13+, Node.js 22+, [Air](https://github.com/air-verse/air), and a TMDB API key.

```sh
go install github.com/air-verse/air@v1.67.4

# terminal 1
air

# terminal 2
cd frontend
npm install
npm run dev
```

Open <http://localhost:5173>. Vite proxies `/api` and `/health` to the Go server on `:8080`; Air rebuilds and restarts the Go server on Go/SQL/env changes, loading `.env.development` then `.env` (values in `.env` win). Docker is not needed for the dev loop — use it on port 8080 only for production-like checks.

See [docs/development.md](docs/development.md) for inspecting the dev database, resetting it after baseline changes, and regenerating the schema diagram.

## Checks

```sh
go test ./...
cd frontend && npm test && npm run build
npx playwright install chromium && npm run test:e2e
```

Tests use BDD-style Given/When/Then names for domain and application behaviour. See [SPEC.md](SPEC.md) for the v0.1 product and engineering scope.

## License

Visto is source-available under the [PolyForm Noncommercial License 1.0.0](LICENSE). Personal and family self-hosting is permitted; commercial use requires a separate licence from the copyright holder.
