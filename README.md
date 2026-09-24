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

## Development

Requirements: Go 1.22+, Node.js 22+, and a TMDB API key for metadata search.

```sh
# terminal 1
VISTO_TMDB_API_KEY=your_tmdb_api_key go run ./cmd/server

# terminal 2
cd frontend
npm install
npm run dev
```

The Vite server proxies `/api` requests to the Go server on port 8080.

## Checks

```sh
go test ./...
cd frontend && npm test && npm run build
```

The project uses BDD-style Given/When/Then test names for domain and application
behaviour. See [SPEC.md](SPEC.md) for the v0.1 product and engineering scope.

## License

Visto is source-available under the [PolyForm Noncommercial License 1.0.0](LICENSE).
Personal and family self-hosting is permitted. Commercial use requires a
separate licence from the copyright holder.
