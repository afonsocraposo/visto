# API spec review

Reviewed on 2026-09-26 against `api/openapi.yaml`, HTTP handlers, Go response types, and the Orval client. This review concerns the REST contract only. MCP and OAuth protocol endpoints have separate documentation.

## Fix contract mismatches

1. **Document response bodies that already exist.** `POST /plays/bulk` returns `[]Play` (`handlers_tracking.go`), but its `201` response has no `content` schema. Orval therefore generates `postPlaysBulk(): Promise<void>`. `GET /export/json` likewise returns structured JSON but generates `Promise<void>`. Add the actual response schemas. For `GET /export/csv`, declare `text/csv` with a string schema only if the generated client is meant to consume it; otherwise state that this is a download endpoint outside the JSON client.

2. **Use the correct library media type.** `LibraryEntry.media` refers to `MediaSearchResult`, which lacks the `id` and `status` fields in the Go `library.Media` response. Point it to the existing `MediaSnapshot` schema. The generated type will then reflect what clients receive.

3. **Describe unsaved movie and show details accurately.** `GET /movies/{tmdbID}` and `GET /shows/{tmdbID}` say they return only titles in the user's library and return `404` otherwise. The handler can fetch an unsaved title from TMDB and return `200` with an empty library item. Update both summaries and the `200` response description. Keep `404` for a title the provider cannot find or load. Consider whether the empty `item` object should instead be absent; that is a handler and client change, so decide it separately.

   **Implementation choice:** Keep the current empty item for client compatibility. The schema allows its empty status and describes where it occurs.

4. **Match optional and nullable fields to JSON output.** `ContinueEntry.next_episode` is required by the spec but has `omitempty` in Go. `ContinueEntry.remaining_episodes` is always serialized but absent from the schema. `TemporarySeasonDetails.episodes` may be serialized as `null` in a show summary because the Go slice is nil, while the spec permits only an array. Check other nil slices with one representative response check, then either initialize empty slices or describe `null` where it is intentional. Prefer normalizing to `[]` for collection fields clients iterate.

5. **Avoid false request and date constraints.** `UpdateLibraryRequest` requires `rating`, while the handler accepts a status-only update. Make `rating` optional and nullable. `MediaSearchResult.release_date` uses `format: date`, but TMDB items without a release date can reach the response as an empty string. Remove the format unless the handler normalizes missing dates to `null` or omits the field.

## Simplify maintenance

- The spec is valid enough for Orval to generate a client, but most operations are dense single-line YAML. Convert only edited operations and schemas to block style so reviewers can see field and response changes. Do not reformat all 988 lines at once.
- `openapi_test.go` checks only whether a registered path string appears somewhere in the file. It cannot detect a missing HTTP method, stale path, incorrect response, or malformed schema. Keep its route check, but add a generated-client freshness check in CI (`npm run generate:api` followed by a clean-tree check) if the generated files remain committed. A small response contract test for the cases above gives more value than a custom OpenAPI framework.
- Reuse the existing `BadRequest` and `Unauthorized` response components when editing operations. Their current description-only form does not describe the actual `{ "error": "..." }` body; add one shared error schema if clients need typed error responses. Do not add `operationId` or new schema aliases across the entire spec solely for style.

## Suggested order

Fix the returned-body and library media schemas first because they already make generated client types wrong. Then correct endpoint descriptions and optional fields. Add the freshness check after regenerating the client.
