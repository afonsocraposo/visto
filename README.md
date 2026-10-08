# Visto

Visto is a self-hosted movie and TV tracker. It stores viewing data locally in
SQLite and uses TMDB only for metadata.

## Run Visto with Docker

You need Docker with Compose and a [TMDB API key](https://www.themoviedb.org/settings/api).
The key lets Visto search for titles and load their artwork and episode details.

1. Make a directory for Visto:

   ```sh
   mkdir visto
   cd visto
   ```

2. Save this as `compose.yaml` in that directory:

   ```yaml
   services:
     visto:
       image: ghcr.io/afonsocraposo/visto:${VISTO_VERSION:-latest}
       ports:
         - "8080:8080"
       environment:
         VISTO_TMDB_API_KEY: ${VISTO_TMDB_API_KEY}
       volumes:
         - visto-data:/data
       restart: unless-stopped

   volumes:
     visto-data:
   ```

3. Save a `.env` file beside `compose.yaml`, replacing the example value with
   your TMDB API key:

   ```dotenv
   VISTO_TMDB_API_KEY=your_tmdb_api_key
   ```

4. Start the app:

   ```sh
   docker compose up -d
   ```

5. Open <http://localhost:8080> and create an account. The first account is the
   administrator.

If the page does not open, run `docker compose ps` and `docker compose logs visto`
from the `visto` directory to check the container. To stop the app, run
`docker compose down`. Your database and backups remain in the `visto-data`
volume. See [configuration](docs/configuration.md) for backups, public access,
and other settings. The repository also has a [full Compose file](compose.yaml)
for optional features. Updates are covered under [Releases](#releases).

## Optional features

- **[Pushover & Web Push alerts](docs/notifications.md)** — per-user notifications for new episodes and movie releases.
- **[Plex watched-content sync](docs/plex-sync.md)** — import watches from Plex via webhook.
- **[ChatGPT MCP connection](docs/mcp.md)** — connect Visto to ChatGPT as an MCP app.

## Releases

Multi-platform images (`linux/amd64`, `linux/arm64`) are published to [ghcr.io/afonsocraposo/visto](https://github.com/afonsocraposo/visto/pkgs/container/visto). [Release Please](https://github.com/googleapis/release-please) opens a release PR from Conventional Commits merged to `main`; merging it cuts the GitHub release and publishes the matching image (`feat:` → minor, `fix:`/`perf:` → patch, breaking changes → minor while below 1.0.0; docs/test/build/CI/refactor/chore commits don't trigger a release).

Pin a deployment to a release with `VISTO_VERSION=0.1.0` in `.env`, then `docker compose pull && docker compose up -d` to upgrade.

> The repository needs "Allow GitHub Actions to create and approve pull requests" enabled under Settings → Actions → General, so the release workflow can open its PR.

## Development

To run the source code locally, install Go 1.25.13+ and Node.js 22+. You also
need a TMDB API key. If you already have a checkout or `.env`, keep them and
skip the matching commands below:

```sh
git clone https://github.com/afonsocraposo/visto.git
cd visto
cp .env.example .env
```

Replace the example `VISTO_TMDB_API_KEY` value in `.env` with your key. Then
install [Air](https://github.com/air-verse/air):

```sh
go install github.com/air-verse/air@v1.67.4
```

Start the backend and frontend in separate terminals, both from the repository
directory:

```sh
# Terminal 1: backend
air
```

```sh
# Terminal 2: frontend
cd frontend
npm ci
npm run dev
```

Open <http://localhost:5173>. Air runs the backend on port 8080 and reloads it
when backend files change. Vite runs the frontend on port 5173. Stop both with
Ctrl+C. Do not run the Docker app at the same time: it also uses port 8080.

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
