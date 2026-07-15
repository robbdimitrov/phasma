# Security

## Session Model

- Session tokens are generated with `crypto/rand` as 21 random bytes encoded
  with unpadded base64url (`[A-Za-z0-9_-]{28}`). Standard base64 characters `+`,
  `/`, and padding `=` are rejected.
- The raw session token is never stored. It is HMAC-SHA256 hashed with
  `SESSION_HASH_SECRET`; that private HMAC value is the session row's primary
  key and is never exposed through the API.
- Each session also has a random public UUID used for listing and remote
  revocation. Exposing this identifier does not expose the cookie token or its
  HMAC.
- The `session` cookie is `HttpOnly`, `SameSite=Strict`, `Secure` (when TLS is
  detected or `TRUST_PROXY=true` and `X-Forwarded-Proto: https`), `Path=/`,
  `Max-Age=604800` (7 days).
- On every authenticated request, `RefreshSession` refreshes the sliding TTL
  only when the session is within the inner half of the TTL window
  (`expires_at < now + TTL/2`); sessions in the outer half are still validated
  but not re-stamped. Sessions expire 7 days after last use or 30 days (720 h,
  configurable via `SESSION_ABSOLUTE_TTL_HOURS`) after creation, whichever comes
  first.
- A background goroutine sweeps expired sessions from PostgreSQL every hour.
- Password changes revoke all other sessions for the user atomically in a single
  transaction.
- Each user is limited to 100 sessions. Session creation is serialized per user
  and removes the oldest excess rows, bounding listing and storage work.
- Active-session listing and revocation derive the user and current token from
  authenticated request state. Remote revocation uses an ownership-constrained
  database delete, returns the same not-found result for missing and unowned
  UUIDs, and refuses to revoke the current session. Current-session termination
  remains the logout operation.

## Password Policy

- Minimum 8 characters, maximum 128 characters.
- Hashed with Argon2id PHC format: `memory=19456 KiB`, `iterations=2`,
  `parallelism=1`, `saltSize=16 bytes`, `hashSize=32 bytes`.
- Concurrent hash operations are bounded by a semaphore (default 4, configurable
  via `ARGON_MAX_CONCURRENCY`) to cap memory usage (~19 MiB per hash). Requests
  waiting for a hash slot honor request cancellation.
- Hashes are upgraded silently to current parameters on next login
  (`NeedsRehash` check).
- A pre-computed decoy hash is verified when the email is not found, preventing
  timing-oracle user enumeration.
- The length policy is enforced when a password is set (registration, change),
  not at login: login always fails the same way (401, generic message) for any
  wrong or out-of-policy password, so the response can't reveal why it failed.

## Ownership Rules

| Operation             | Enforcement                                                                                                                                       |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| Update user profile   | `userId` path param must equal session `userID` (checked in handler)                                                                              |
| Delete post           | `WHERE public_id = $1 AND user_id = $2` (atomic in DB)                                                                                            |
| Delete comment        | `WHERE public_id = $1 AND post_id = ... AND (user_id = $3 OR post_id IN (posts owned by $3))` — comment author or the post's owner (atomic in DB) |
| Revoke remote session | Atomic delete constrained by `public_id` and authenticated `user_id`; missing and unowned IDs both return 404                                     |
| Follow/unfollow self  | Blocked at service layer (`currentUserID == targetUserID`)                                                                                        |
| Read email field      | Stripped in handler unless requester is the user                                                                                                  |
| Upload image          | Registered to `userID`; consumed atomically at post/avatar creation                                                                               |

## What Is Protected

- Routes are registered on one of two `http.ServeMux` instances in
  `internal/app/routes.go`: `public` or `protected`. `httpx.RequireSession`
  wraps `protected` and selected public-mux literals that must outrank public
  wildcard routes, rejecting requests without a valid session cookie.
- Public read routes that personalize for a signed-in viewer (e.g.
  `GET /posts/{publicId}`, `GET /users/{username}`) are wrapped with
  `httpx.OptionalSession` instead: it populates the viewer id in context when
  a valid session is present but never rejects the request, so anonymous and
  authenticated visitors reach the same handler while `liked`, `isFollowing`,
  and email visibility differ correctly by viewer identity. All
  state-changing routes (create/update/delete, likes, comments, follows) remain
  on `protected`; discovery, social-list, and comment-list reads such as
  `GET /posts/popular`, `GET /users/{username}/likes`, follower/following
  lists, and `GET /posts/{publicId}/comments` require a session. Liked-post
  lists are authenticated social reads, not owner-only private data.
- Adding a new public route requires explicit registration on `public` and
  justification for bypassing `RequireSession`.

## Rate Limiting

- Token bucket per policy, keyed by user id > session cookie > client IP.
- Implemented as a Lua script on Dragonfly (atomic HMGET + HMSET + PEXPIRE).
- `TRUST_PROXY=true` honors only parseable `X-Forwarded-For` IP values for IP
  extraction; invalid values fall back to `RemoteAddr`. Enable it only behind an
  upstream proxy that overwrites forwarding headers.
- Failure mode: configurable via `RATE_LIMIT_FAIL_OPEN` (default false →
  fail-closed on Dragonfly error); fail-closed returns HTTP 503.
  `RATE_LIMIT_FAIL_OPEN` applies only to the token-bucket rate limiter.
- Login throttle uses separate per-IP (5 failures) and per-email (50 failures)
  counters in Dragonfly with 15 min TTL, cleared on successful login. When
  Dragonfly is unavailable, the login throttle always fails open regardless of
  `RATE_LIMIT_FAIL_OPEN`.

## Upload Security

- Frontend upload and avatar actions reject files over 1 MB and MIME types
  outside JPEG, PNG, GIF, and WEBP before proxying bytes to the backend. These
  checks are usability and load-shedding controls; the backend remains the
  authoritative validator.
- File content is validated against magic bytes (not extension or MIME header):
  JPEG `\xff\xd8\xff`, PNG `\x89PNG\r\n\x1a\n`, GIF `GIF8`, WEBP `RIFF....WEBP`.
- Multipart read is bounded to 1 MB + 1 byte; files exceeding 1 MB are rejected
  before any further processing.
- All accepted images are decoded and re-encoded to JPEG at 90% quality.
  Re-encoding strips embedded metadata (EXIF, GPS coordinates, ICC profiles) and
  eliminates polyglot payloads that pass the magic byte check. Re-encoded output
  is rejected if it exceeds 1 MB.
- Pixel dimensions are checked via `image.DecodeConfig` before full decode;
  images exceeding 25 MP (e.g. 5000×5000) are rejected to guard against
  decompression bombs.
- All stored images have content type `image/jpeg` regardless of the original
  upload format.
- Filenames are 32 lowercase hex characters generated from `crypto/rand`.
- File serving validates the path segment against the 32-char hex pattern before
  proxying.

## HTTP Security Headers (backend)

The API only ever emits JSON (`WriteJSON`/`WriteMessage`), so the CSP stays
fully locked down rather than carrying a browser-app baseline this origin
doesn't need. No `Strict-Transport-Security`: this deployment has no TLS
termination (local k3s only), and sending it over plain HTTP would be a false
guarantee to clients.

- `X-XSS-Protection: 0` (legacy auditor disabled; CSP takes over)
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: no-referrer`
- `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'`

## HTTP Security Headers (frontend)

No `Strict-Transport-Security`, for the same reason as the backend.

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Cross-Origin-Opener-Policy: same-origin`
- `Permissions-Policy: camera=(), microphone=(), geolocation=(), payment=(), usb=(), bluetooth=()`
- SvelteKit nonce-based `Content-Security-Policy`:
  `default-src 'self'; script-src 'self'` plus per-request nonce;
  `style-src 'self'`;
  `style-src-attr 'unsafe-hashes' 'sha256-S8qMpvofolR8Mpjy4kQvEm7m1q8clzU4dfDH0AmvZjo='`
  (SvelteKit's route announcer mounts with a fixed inline style attribute a nonce
  cannot cover; scoping the hash to `style-src-attr` keeps `style-src` strict, and
  `unsafe-hashes` — required for hashes to apply to attributes — permits only that
  exact string);
  `img-src 'self' data: blob:; connect-src 'self'; font-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; frame-src 'none'`

The uploads proxy route (`/uploads/[key]`) also sets
`Cross-Origin-Resource-Policy: cross-origin` to allow image embedding from other
origins.

## Search Filter Injection Prevention

- Hashtag filter values are validated against `^[A-Za-z0-9_]{1,50}$` before
  being interpolated into Meilisearch filter strings.
- All other query parameters are passed as structured JSON fields, never as raw
  filter fragments.

## CSRF Protection

- `OriginGuard` middleware rejects mutating requests where `Origin` does not
  match the request host.
- `SameSite=Strict` cookie blocks cross-site form submission.

## Meilisearch Key Isolation

- The master key is used once at startup to provision a one-year scoped API key
  with only `search`, `documents.add`, and `documents.delete` actions on the
  `users`, `posts`, and `hashtags` indexes.
- All subsequent Meilisearch operations use the scoped key; the master key is
  not retained in memory.

## Database Privilege Isolation

- The backend connects as `phasma_app`, which holds SELECT/INSERT/UPDATE/DELETE
  on all tables and USAGE/SELECT on all sequences — no DDL or superuser
  privileges.
- Kafka Connect syncs Meilisearch data as `phasma_connect`, a read-only role
  scoped to `outbox`, `users`, `posts`, `hashtags`, and `post_hashtags` — it
  cannot read `sessions`, `notifications`, or any other table.
