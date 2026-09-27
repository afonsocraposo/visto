# Ponytail review

Reviewed on 2026-09-25. Scope: Go server, SQLite storage, HTTP and MCP surfaces, TMDB adapter, React client, service worker, build scripts, tests, and deployment files. This is a list of suggested changes, not an implementation plan for new features. The existing `PROJECT-IMPROVEMENT-PLAN.md` records completed work; do not reopen those items without a new failure.

## Do first

1. **Bound the two remaining TMDB detail caches.** `Movie` and `Episode` insert into `movieCache` and `episodeCache` without deleting expired entries or limiting their count (`internal/infrastructure/tmdb/details.go`). The show, person, season, search, and related caches already have limits. Reuse the same simple expiry and size pattern for these two maps. Add one focused check that inserts more than the limit and confirms old entries leave. This closes a real memory growth path without a cache package.

2. **Keep the signed-in screen when session lookup has a temporary error.** In `frontend/src/app/App.tsx`, the `/api/v1/me` query maps every non-2xx response to `null`, and the render path sends the user to `AuthGate`. A 500 response or lost connection therefore looks like logout. Return `null` only for an authentication failure; throw for server and network failures. Show a retry state for that error. Keep the existing service worker cache behavior. One test for 401 versus 500 is enough.

3. **Use one frontend request implementation.** `frontend/src/lib/api.ts` and `frontend/src/lib/orvalMutator.ts` both implement fetch, error decoding, offline events, and empty-response handling. They can drift when response behavior changes. Make the generated API mutator call the existing request function, or extract the shared request code into one small function. Keep the existing error messages and public signatures. Do this when either file next needs a change; it is a small maintenance cleanup, not a reason to rewrite every API call.

4. **Limit JSON request bodies at the HTTP boundary.** Most REST handlers decode `r.Body` directly, while Plex and MCP requests already have size limits. A large or slow JSON upload can occupy a server connection and decoder. Apply one modest limit to `/api/v1` request bodies before handlers run, with the existing Plex limit preserved. Add a test for an oversized login or bulk watch request. `cmd/server/main.go` sets a header timeout but no body read timeout; assess a read timeout with the same change, accounting for MCP's separate transport behavior.

5. **Count only invalid credentials as login failures.** `auth.Service.Login` can fail when session creation or token generation fails. `internal/presentation/http/handlers_auth.go` currently counts every error as a failed attempt and returns 401. Count `auth.ErrInvalidCredentials` only; return a server error for other failures. Also preserve repository lookup errors in `AuthenticateCredentials` so database failures do not become invalid credentials. One small service and handler check should cover this.

6. **Make CSV exports safe to open in a spreadsheet.** `internal/presentation/http/handlers_export.go` writes media titles directly into CSV cells. CSV quoting does not stop spreadsheet formulas in titles that start with `=`, `+`, `-`, or `@`. Escape those leading characters in spreadsheet-facing CSV fields, and check one formula-like title. Leave JSON exports unchanged so they retain the exact title.

## Do when touching the area

7. **Remove the unused runtime package.** `drizzle-orm` is declared in `frontend/package.json` but has no source import. The database diagram uses `better-sqlite3`, and Studio uses `drizzle-kit`. Remove `drizzle-orm` and update the lockfile if `npm ci`, `db:diagram`, and `db:studio` still work. Keep those two tools while they serve current scripts.

   **Implementation check:** `db:studio` fails with “Please install latest version of drizzle-orm” when this package is removed. Keep the dependency while Studio remains a supported command.

8. **Check the initial bundle before further splitting.** The current build still reports a 509.61 kB initial JavaScript chunk and 251.61 kB CSS. Existing route level lazy loading works, so avoid another routing abstraction. First inspect what remains in the initial chunk; move a feature import only if it reduces initial load enough to matter. Do not raise the warning threshold just to hide the number.

9. **Keep API typing on one path as code changes.** The frontend uses generated OpenAPI calls in search and media detail queries, while many other screens use manually typed `api.get<T>` calls. This makes contract drift easier, but a wholesale conversion would create a large diff with little immediate value. Use generated calls for new or materially changed endpoints. Keep the route coverage test in `internal/presentation/http/openapi_test.go`.

## Leave alone for now

- Keep SQLite as one process with `SetMaxOpenConns(1)` and WAL. Nothing in this review shows a throughput problem that justifies a queue, second database, or cache server.
- Keep the current Go application layer and repository interfaces where they isolate HTTP, MCP, and SQLite callers. Removing them now would touch more files than it would save.
- Keep the service worker's account scoped cache and logout cleanup. The existing test covers offline reads, the entry cap, and logout removal.
- Do not add a general cache framework, state library, test framework, or new configuration system for these findings.

## Checks made

- `go test ./...` passed with `GOCACHE` set to a writable temporary directory.
- `npm test` passed: 55 tests.
- `npm run build` passed and produced the bundle sizes above.
- The review did not modify application code. The existing untracked `simkl-tv-show-progress.txt` was left untouched.
