# API

All endpoints are served by the Go backend on port 8080. All responses are JSON.
Error bodies are `{"message": "..."}`.

## Middleware Stack (outermost → innermost)

1. `Recover` — recovers a panic anywhere in the stack and returns a structured
   `500` JSON response instead of an aborted connection.
2. `RequestID` — accepts `X-Request-ID` (max 64 chars; generates a new 16-byte
   hex id if absent or over the limit) and echoes it in the response header.
3. `Logger` — structured JSON request log with method, route pattern, path,
   status, duration.
4. `SecurityHeaders` — sets `X-XSS-Protection: 0`,
   `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
   `Referrer-Policy: no-referrer`, and
   `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'`.
   No `Strict-Transport-Security`: this deployment has no TLS termination.
5. `OriginGuard` — for POST/PUT/PATCH/DELETE, rejects requests where `Origin`
   header is present but does not match the request host.
6. `RateLimit` — token bucket via Dragonfly Lua script; key priority: user id >
   session cookie > client IP.
7. `RequireSession` — validates `session` cookie; refreshes sliding TTL; injects
   `userID` into context. It wraps protected routes plus protected literals that
   must be registered on the public mux to outrank public wildcards. Public
   routes listed below bypass it; personalized public reads use
   `OptionalSession`.

## Rate Limit Policies

| Policy    | Endpoints                                                     | Burst | Rate (req/s) |
| --------- | ------------------------------------------------------------- | ----- | ------------ |
| strict    | POST /sessions, POST /users, POST /uploads                    | 5     | 0.2          |
| typeahead | GET /users/search, GET /hashtags/search, GET /search          | 20    | 5            |
| read      | GET/HEAD (all others)                                         | 120   | 2            |
| mutation  | POST/PUT/PATCH/DELETE (all others)                            | 30    | 1            |
| exempt    | GET /health, GET /health/background, GET /metrics, GET /ready | —     | —            |

Defaults are overridable via env vars `RATE_LIMIT_{POLICY}_{BURST,RATE}`.

## Endpoint Inventory

### Public (no auth required)

| Method | Path                        | Purpose                                                                                              |
| ------ | --------------------------- | ---------------------------------------------------------------------------------------------------- |
| GET    | /health                     | Liveness check — 204 No Content                                                                      |
| GET    | /health/background          | Background pipeline health/progress snapshot                                                         |
| GET    | /metrics                    | Prometheus text metrics for background pipeline health                                               |
| GET    | /ready                      | Readiness check — pings PostgreSQL and configured background pipelines (2 s timeout), 204 No Content |
| POST   | /users                      | Create account — returns `{"username": "..."}` on 201                                                |
| POST   | /sessions                   | Login — sets `session` cookie, returns `{"username": "..."}` on 201                                  |
| GET    | /uploads/                   | Serve uploaded file blob                                                                             |
| GET    | /users/{username}           | Get user by username                                                                                 |
| GET    | /posts/{publicId}           | Get single post                                                                                      |
| GET    | /users/{username}/posts     | List user's posts (cursor-paginated)                                                                 |

These routes read an optional viewer id from the session cookie when present
(for `liked`/ownership flags) but do not require one — an anonymous request
succeeds and degrades gracefully.

### Protected (session cookie required)

#### Sessions

| Method | Path                  | Purpose                                       |
| ------ | --------------------- | --------------------------------------------- |
| GET    | /sessions             | List the authenticated user's active sessions |
| DELETE | /sessions             | Logout — clears session cookie                |
| DELETE | /sessions/{sessionId} | Revoke one remote session by public UUID      |

`GET /sessions` returns active sessions with the current session first, then
the rest newest first:

```json
{
  "sessions": [
    {
      "id": "01904d2e-7f4d-7c33-ae21-2f94737eaa10",
      "created": "2026-06-22T12:00:00Z",
      "expiresAt": "2026-06-29T12:00:00Z",
      "current": true
    }
  ]
}
```

The `id` field is the session's public UUID, not the raw cookie token or its
private HMAC database key. The list includes only sessions owned by the
authenticated user that remain within both the sliding expiry and absolute
lifetime. `expiresAt` is the earlier of those two limits. Accounts retain at
most 100 sessions, so the response is bounded. Responses are `200`; repository
failures return `500`.

`DELETE /sessions/{sessionId}` validates `sessionId` as a UUID and deletes only
a session owned by the authenticated user. It returns `204` on success, `400`
for a malformed UUID, `404` when the session is missing or belongs to another
user, `409` when it identifies the current session, and `500` on repository
failure. Use `DELETE /sessions` to terminate the current session.

#### Users

| Method | Path                     | Purpose                           |
| ------ | ------------------------ | --------------------------------- |
| GET    | /users/me                | Get current authenticated user    |
| PUT    | /users/me                | Update profile or change password |
| POST   | /users/{username}/follow | Follow a user                     |
| DELETE | /users/{username}/follow | Unfollow a user                   |
| GET    | /users/{username}/followers | List followers (cursor-paginated) |
| GET    | /users/{username}/following | List following (cursor-paginated) |

`GET /users/{username}` is public — see the Public section above. Follower and
following lists require a session; public profiles expose only aggregate counts.

#### Discovery

| Method | Path             | Purpose                               |
| ------ | ---------------- | ------------------------------------- |
| GET    | /users/suggested | Get up to 5 suggested users to follow |
| GET    | /posts/popular   | Get up to 20 popular posts from the last 7 days |

`GET /users/suggested` returns users with at least one follower or post,
ordered by `follower_count` descending then `post_count` descending,
excluding users the authenticated user already follows and the authenticated
user themselves. Response:

```json
{"items": [<user>]}
```

`GET /posts/popular` returns posts from the last 7 days ordered by like count
descending, up to 20 results. Response:

```json
{"items": [<post>]}
```

#### Feed

| Method | Path  | Purpose                                              |
| ------ | ----- | ---------------------------------------------------- |
| GET    | /feed | Get the authenticated user's feed (cursor-paginated) |

`GET /feed` returns posts from the feed table for the authenticated user,
ordered `(created DESC, id DESC)`. Response shape matches the post list shape.
Returns an empty items array (not an error) when the feed is empty.

#### Posts

| Method | Path                    | Purpose                    |
| ------ | ----------------------- | -------------------------- |
| POST   | /posts                  | Create post from an upload |
| DELETE | /posts/{publicId}       | Delete own post            |
| POST   | /posts/{publicId}/likes | Like a post                |
| DELETE | /posts/{publicId}/likes | Unlike a post              |
| GET    | /users/{username}/likes | List user's liked posts (cursor-paginated) |

`GET /posts/{publicId}` and `GET /users/{username}/posts` are public — see the
Public section above. Liked-post lists require a session but not profile
ownership; any signed-in viewer can open another user's liked-post list. Public
profiles expose only aggregate like counts.

#### Comments

| Method | Path                                   | Purpose                                           |
| ------ | -------------------------------------- | ------------------------------------------------- |
| GET    | /posts/{publicId}/comments             | List comments on a post (cursor-paginated)       |
| POST   | /posts/{publicId}/comments             | Create a comment                                  |
| DELETE | /posts/{publicId}/comments/{commentId} | Delete a comment (its author or the post's owner) |

Direct post pages expose the post and aggregate comment count publicly. Comment
lists, creation, and deletion require a session.

#### Uploads

| Method | Path     | Purpose                                    |
| ------ | -------- | ------------------------------------------ |
| POST   | /uploads | Upload an image file; returns `{filename}` |

#### Search

| Method | Path                     | Purpose                                                                                    |
| ------ | ------------------------ | ------------------------------------------------------------------------------------------ |
| GET    | /users/search?q=         | Typeahead user search (up to 8 results)                                                    |
| GET    | /hashtags/search?q=      | Typeahead hashtag search (up to 8 results)                                                 |
| GET    | /search?q=&type=&cursor= | Full search — type: `users`, `posts`, `hashtags`, or `all` (blended); requires Meilisearch |
| GET    | /search/recent           | List the authenticated user's recent searches (newest first, capped at 10)                 |
| POST   | /search/recent           | Record a recent search: `{type: "users"\|"hashtags"\|"posts", reference}`                  |
| DELETE | /search/recent/{id}      | Remove one recent search                                                                    |
| DELETE | /search/recent           | Clear all of the authenticated user's recent searches                                       |

#### Notifications

| Method | Path                        | Purpose                                                          |
| ------ | --------------------------- | ---------------------------------------------------------------- |
| GET    | /notifications              | List notifications for the authenticated user (cursor-paginated) |
| GET    | /notifications/unread-count | Get the authenticated user's unread notification count           |
| PUT    | /notifications/{id}/read    | Mark one notification as read                                    |

`GET /notifications` returns cursor-paginated notifications ordered
`(created DESC, id DESC)`:

```json
{
  "items": [
    {
      "id": "01904d2e-7f4d-7c33-ae21-2f94737eaa10",
      "actorUsername": "alice",
      "actorName": "Alice",
      "actorAvatar": null,
      "type": "like",
      "entityId": "01904d2e-7f4d-7c33-ae21-2f94737eab20",
      "read": false,
      "created": "2026-06-22T12:00:00Z"
    }
  ],
  "nextCursor": null
}
```

`actorUsername`, `actorName`, and `actorAvatar` describe the user who
triggered the notification (joined from `notifications.actor_id`).

`GET /notifications/unread-count` returns `{"count": <int>}`, the number of
the authenticated user's notifications with `read = false`.

Notification types: `like` (entityId = post public_id), `comment` (entityId =
comment id), `follow` (entityId = actor user id as string).

`PUT /notifications/{id}/read` requires `id` to be a valid UUID. Returns `204`
on success, `400` for an invalid UUID, `404` when the notification does not
exist or belongs to another user, and `500` on repository failure. Ownership is
enforced in the UPDATE query.

## Pagination Model

- All paginated endpoints accept `cursor` (base64url-encoded JSON
  `{created, id}`) and `limit` (1–50, default 10; values above 50 are silently
  clamped to 50) query parameters.
- Response shape: `{"items": [...], "nextCursor": "<string or null>"}`.
- `page` parameter is rejected (returns 400).
- Ordering: `(created DESC, id DESC)` throughout.

## Search Endpoint (`GET /search`)

- `type=posts`: full-text search on description and username; supports
  `q=#hashtag` to filter by hashtag (exact match via Meilisearch filter). Items
  include `filename` for rendering a thumbnail.
- `type=users`: full-text search on username and name. Items include `name` and
  `avatar` (nullable) alongside `username`.
- `type=hashtags`: full-text search on name. Items are `{name, postCount}`.
- `type=all`: a single blended, ranked page mixing all three entity types. Not
  currently called by the frontend (the search page's typeahead makes
  separate per-type requests and its results page is posts-only — see
  `docs/frontend.md`); kept as a general-purpose blended search mode. Roughly
  a 20/60/20 users/posts/hashtags split per page (`computeBlendTargets`, min 1
  user/1 hashtag once `limit >= 3`).
  Items are `{"type": "users"|"posts"|"hashtags", "item": <the type's normal
item shape>}`. Users the viewer follows are boosted to the front of the
  page's user results (never across pages). Cursor encodes independent
  per-index offsets (opaque to the client). A page can legitimately return
  fewer than `limit` items when two entity types are simultaneously scarce
  (no result is ever skipped, duplicated, or fabricated). Also supports
  `q=#hashtag` — same exact-match filter as `type=posts`, applied only to the
  posts portion of the blend.
- For `type=users|posts|hashtags`, cursor encodes a single Meilisearch offset
  (base64-encoded integer string); page size defaults to 20 and accepts an
  optional `limit` (1–50, clamped) for smaller previews.
- Returns 503 if Meilisearch is not configured (all `type` values).
- Query must be 1–50 UTF-8 runes.

`GET /users/search` and `GET /hashtags/search` typeahead results share the
same `avatar`/`name`/`postCount` fields as the corresponding `/search` item
shapes.

## Recent Searches (`/search/recent`)

`GET /search/recent` returns the authenticated user's search history, newest
first, capped at 10 entries, in the same `{"type", "item"}` shape as
`GET /search?type=all`'s blended items (plus an `id`):

```json
[
  { "id": "01904d2e-...", "type": "users", "item": { "username": "alice", "name": "Alice", "avatar": null } },
  { "id": "01904d2e-...", "type": "hashtags", "item": { "name": "cats", "postCount": 12 } },
  { "id": "01904d2e-...", "type": "posts", "item": "sunset beach" }
]
```

For `type: "posts"`, `item` is the raw, verbatim query text the user
submitted (including any `@`/`#` prefix) rather than a post — see
`docs/business-rules.md` for why suggestion clicks and free-text submissions
are recorded differently.

`POST /search/recent` records `{type: "users"|"hashtags"|"posts", reference}`.
`reference` is validated the same way the corresponding value is validated
everywhere else it's accepted (username shape for `users`, hashtag name shape
for `hashtags`, 1–50 UTF-8 runes for `posts`); an invalid `type` or `reference`
returns 400. Recording the same `(type, reference)` again bumps it to the top
instead of duplicating it; the list is silently trimmed back to 10 entries.

`DELETE /search/recent/{id}` requires `id` to be a valid UUID, returns 400
otherwise. Returns 204 on success, 404 when the entry does not exist or
belongs to another user (ownership is enforced in the `DELETE` query).

`DELETE /search/recent` clears all of the authenticated user's recent
searches and returns 204.

## User Object Shape

User objects returned by `/users/me`, `/users/{username}`, authenticated
follower/following lists, and suggested users share this shape:

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Alice",
  "username": "alice",
  "email": "alice@example.com",
  "avatar": null,
  "bio": null,
  "posts": 12,
  "likes": 34,
  "followers": 5,
  "following": 3,
  "isFollowing": false,
  "created": "2026-01-01T00:00:00Z"
}
```

`id` is the user's public UUID (`public_id` column). The internal integer
primary key is never exposed. `email` is stripped from user responses unless the
requester's session belongs to that user.
