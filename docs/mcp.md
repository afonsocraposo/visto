# ChatGPT MCP connection

Visto's MCP endpoint is `/mcp`. Set `VISTO_PUBLIC_URL` to the canonical HTTPS
origin used to reach the instance (e.g. `https://visto.example.com`) — OAuth
discovery uses this exact origin, so it must match the external address
exposed by your reverse proxy or secure tunnel. Keep the `/mcp`, `/oauth/`,
and `/.well-known/` paths reachable through that proxy. The server supports
PKCE authorization, separate read and write permissions, and per-user Visto
accounts.

`VISTO_TRUSTED_PROXY_CIDRS` is optional for MCP behind a reverse proxy; set it
to the proxy's IP range if you need visitor-IP-aware rate limits (Visto only
trusts forwarded client/HTTPS headers from those ranges). OAuth codes and old
tokens are cleaned up in bounded daily batches — change the interval with
`VISTO_OAUTH_CLEANUP_INTERVAL`.

## Connecting

In ChatGPT web, enable developer mode, create a custom MCP app, and enter
`https://visto.example.com/mcp` as its endpoint. ChatGPT discovers Visto's
OAuth endpoints and asks each user to sign in with their Visto account and
approve access. Visto uses the identity tied to that user's token, not a
`user_id` passed in a tool call. See
[OpenAI's MCP app guide](https://help.openai.com/en/articles/12584461-developer-mode-and-mcp-apps-in-chatgpt).

## Tools

- `get_show_episodes` lists episode IDs and watched state.
- `mark_episodes_through` marks missing released episodes up to a chosen
  episode, even if that episode is already watched.
- `mark_season_watched` and `mark_selected_episodes_watched` cover other bulk
  updates; `mark_selected_episodes_unwatched` corrects mistakes. Bulk actions
  skip existing plays, so repeating one doesn't create rewatch history.
- `get_show_progress` reports earlier gaps separately from `is_caught_up`,
  which only refers to episodes after the furthest watched episode.
- `get_library` lists saved media, most recently updated first, with filters
  by `status` and `media_type`. It is paginated like the REST library list:
  it returns up to `limit` entries (1–100, default 30) as
  `{ "items": [...], "next_cursor": "...", "total_count": 347 }`. When
  `next_cursor` is not null, call `get_library` again with the same filters
  and `cursor` set to that value to get the next page. A cursor only works
  with the filters it came from.
- `remove_media` (destructive) removes a movie or show from the authenticated
  user's library along with that user's plays, ratings, and activity for the
  title. Other users' records and shared catalog data are untouched.

The equivalent REST call for watch-through is
`POST /api/v1/shows/{showID}/episodes/watch-through` with `season_number` and
`episode_number`.

## REST pagination

List responses for `/api/v1/library`, `/api/v1/plays`,
`/api/v1/shows/{showID}/episodes`, `/api/v1/seasons/{seasonID}/episodes`, and
`/api/v1/users` return `{ "items": [...], "next_cursor": null }` instead of a
bare array. Pass `limit` (1–100, default 30) and then pass `next_cursor` back
as `cursor` for the next page. Cursors are tied to the endpoint and its
filters. Library lists also accept `sort`, `status`, and `media_type`; play
history accepts `media_id` or `episode_id`.

> Existing REST clients must update their response parsing to the
> `items`/`next_cursor` shape when they upgrade the server.
