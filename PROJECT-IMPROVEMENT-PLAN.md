# Visto Project Improvement Plan

Reviewed on 2026-09-24. This is a prioritized plan based on the current
repository, not a commitment to a release date.

## Implementation status

Completed on 2026-09-24:

- Connected-app controls: users can inspect and revoke one or all OAuth grants.
- Request controls: bounded login and OAuth throttles plus explicit trusted
  proxy CIDRs for forwarded client and HTTPS headers.
- OAuth lifecycle: bounded cleanup of expired codes and retained old tokens.
- Offline privacy: signing out removes the signed-in account's cached data.
- Frontend loading: details, people, library, feed, and discovery load by route;
  the initial JavaScript chunk is below the 500 kB warning threshold.
- API contract: the public trending route is documented and a test checks that
  REST route registrations stay represented in OpenAPI.
- Shutdown: background workers share cancellation and are awaited before SQLite
  closes.
- Operations: CI runs static checks, restore coverage, and vulnerability checks;
  dependency updates are proposed weekly for review; restore instructions are
  documented. The Go runtime baseline is 1.25.13 to include current
  standard-library security fixes.

## Summary

Visto has a solid base: its Go application is split into domain, application,
infrastructure, and presentation layers; SQLite schema changes are versioned
and checksummed; the frontend has feature-level modules; and CI runs backend,
frontend, end-to-end, and container checks.

The next improvements should focus on account security and operations before
adding more product features. The production build also reports a large initial
JavaScript chunk, and the OpenAPI document has a small route-coverage gap.

## Priority 1 — Account security and data lifecycle

### 1. Add connected-app controls for OAuth grants

Users can authorize OAuth clients and clients can call the revocation endpoint,
but the Profile UI does not let a user review or revoke connected clients.
Provide a list of authorized apps with scopes and last-used/created details,
plus per-app and revoke-all actions. Revocation must invalidate access and
refresh tokens immediately.

**Acceptance checks**

- A user can see only their own grants.
- Revoking one app blocks its existing access and refresh tokens.
- Revoke-all does not affect another user's grants.
- UI and API tests cover read-only and read/write grants.

### 2. Bound request throttling and trust proxy addresses explicitly

The OAuth throttler stores buckets by remote address and scans its full map for
each request. Under traffic from many distinct addresses, memory use and request
cost can grow. The login throttler also sees the direct peer address, which can
make all users appear to share one address behind a reverse proxy.

Replace this with a bounded throttling design, periodic expiry, and explicit
trusted-proxy configuration. Only accept forwarded client addresses from
configured proxies; never trust arbitrary forwarded headers.

**Acceptance checks**

- Bucket count has a fixed upper bound or safe eviction policy.
- Cleanup work does not scan all keys on every request.
- Spoofed forwarded headers do not bypass limits.
- Proxy deployments can rate-limit individual clients when configured.
- Tests cover limits, expiry, eviction, and proxy trust.

### 3. Clean up expired OAuth records

Consumed authorization codes are removed, but expired unused codes and old
expired or revoked access/refresh tokens remain in SQLite. Add a bounded
maintenance task that deletes records only after a safe audit-retention window.

**Acceptance checks**

- Active and unexpired tokens are never removed.
- Expired codes and expired/revoked tokens are eventually deleted.
- Cleanup processes a bounded batch and is safe to retry.
- Tests use a controlled clock and verify the retention boundary.

### 4. Define offline-cache behavior on logout

The service worker scopes cached responses by account and clears the active
account pointer on logout. The account's cached responses remain in the browser
cache. Make the privacy behavior explicit and clear that account's cached data
on logout, or provide a clearly explained opt-in to retain it for offline use.

**Acceptance checks**

- Logout removes the active account pointer and follows the selected cache
  policy for that account.
- A second account cannot receive the first account's cached responses.
- Tests cover logout, account switching, and offline reads.

## Priority 2 — Performance and API contract

### 5. Split the initial frontend bundle

The current production build emits a JavaScript chunk of about 687 kB before
gzip and reports that it exceeds Vite's 500 kB warning threshold. Load less-used
screens and large feature code on demand, starting with detail, people, and
profile/settings routes. Keep shared app chrome and core navigation eager.

**Acceptance checks**

- The initial chunk is below the current warning threshold, or the remaining
  size is explained with measured data.
- Route navigation still works with direct links and browser back/forward.
- Production build and Playwright smoke tests pass.

### 6. Keep the OpenAPI contract aligned with routes

`api/openapi.yaml` documents the REST API, but the registered
`GET /api/v1/public/trending` route is not listed. Add it and add a CI check that
detects undocumented or stale routes. Document the separate MCP and OAuth
protocol endpoints in a suitable document rather than treating them as REST.

**Acceptance checks**

- Every public REST route is represented in OpenAPI, except explicitly listed
  infrastructure routes such as `/health`.
- CI validates the OpenAPI document and route coverage.
- Auth requirements and error responses match handler behavior.

## Priority 3 — Runtime operations and release confidence

### 7. Coordinate background-worker shutdown

The server starts catalog refresh, backup, and optional notification workers.
On termination, HTTP shutdown runs before deferred cancellation, and the
process does not wait for workers to stop. Give all workers a shared lifecycle:
cancel them when shutdown begins, wait for in-flight work within the shutdown
deadline, then close SQLite.

**Acceptance checks**

- Termination stops accepting HTTP requests and signals workers promptly.
- In-flight database work finishes or exits on cancellation before the database
  closes.
- A timeout still permits clean process exit.
- Tests cover normal and timed-out shutdown.

### 8. Add operational restore and security checks

CI already runs Go tests, frontend tests/build, Playwright, and a Docker build.
Extend it with `go vet ./...` and a backup/restore integration check. Document a
restore procedure alongside the existing backup instructions. Add dependency
vulnerability checks with a reviewed update policy.

**Acceptance checks**

- CI validates a database backup by restoring it to a temporary database and
  checking schema and sample user data.
- The restore guide explains how to stop Visto, preserve the original data,
  restore, and verify startup.
- Dependency alerts are actionable and do not auto-upgrade production
  dependencies without review.

## Suggested order

1. OAuth connected-app management and OAuth record cleanup.
2. Bounded throttling and trusted-proxy configuration.
3. Logout cache privacy behavior.
4. Frontend route splitting and initial-load measurement.
5. OpenAPI route validation.
6. Worker shutdown lifecycle and backup-restore validation.

Reassess priorities after the security and performance work. Avoid adding new
infrastructure unless measurements show that the single-process SQLite design
no longer meets the needs of a typical Visto installation.
