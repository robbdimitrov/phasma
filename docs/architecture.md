# Architecture

## Service Topology

| Service    | Role                                                      | Image                                                     |
| ---------- | --------------------------------------------------------- | --------------------------------------------------------- |
| `frontend` | SvelteKit SSR BFF — sole public entry point               | `localhost:5000/phasma/frontend`                          |
| `backend`  | Go HTTP API — all business logic                          | `localhost:5000/phasma/backend`                           |
| `database` | PostgreSQL — primary data store; migration init container | `postgres:18.4-alpine` / `localhost:5000/phasma/database` |
| `storage`  | SeaweedFS — S3-compatible object storage                  | `chrislusf/seaweedfs:3.76`                                |
| `cache`    | Dragonfly — rate limiting and login throttle              | `docker.dragonflydb.io/dragonflydb/dragonfly:v1.25.0`     |
| `search`   | Meilisearch — full-text search                            | `getmeili/meilisearch:v1.11.3`                            |
| `broker`   | Redpanda — Kafka-compatible event broker                  | `docker.redpanda.com/redpandadata/redpanda:v24.3.7`       |
| `connect`  | Redpanda Connect — Meilisearch sync consumer/backfill     | `docker.redpanda.com/redpandadata/connect:4.38.0`         |

## Request Flow

```
Browser → nginx Ingress → frontend:8080 (SvelteKit SSR)
                              │
                              └─ server-side only ─→ backend:8080 (Go HTTP)
                                                          │
                                           ┌──────────────┼──────────────┐
                                        database        storage        cache
                                        (pgx)          (S3 SDK)      (Dragonfly)
                                           │
                                        search
                                        (HTTP)

database (outbox polling) → backend relay → broker
                     │
                     ├── topic: entity-changes ──► connect sync-search → search
                     │                         ──► backend notifications-consumer
                     │                         ──► backend feed-consumer
                     │
                     └── topic: activity ────────► backend notifications-consumer
                                                ──► backend feed-consumer
```

- The browser never calls the backend directly. `connect-src 'self'` CSP
  enforces the boundary.
- The frontend forwards the session cookie from its own cookie jar to the
  backend on every server-side API call.
- Image bytes (`/uploads/*`) stream through the frontend BFF — the browser only
  talks to its own origin.

## Component Responsibilities

### Frontend (SvelteKit)

- Renders all pages server-side via `load` functions.
- Proxies all mutations through form actions using `use:enhance`.
- Forwards `session` cookie to backend; re-emits it to the browser on login.
- Streams image blobs through without buffering.
- Resizes all images client-side before upload via canvas re-encode to JPEG,
  targeting < 900 KB and max 1600 px on the longest side. Canvas encoding strips
  EXIF metadata. The backend hard limit is 1 MB.

### Backend (Go)

- Stateless HTTP API; all state lives in PostgreSQL, S3, Dragonfly, and
  Meilisearch.
- Auth boundary: `internal/app/routes.go` registers routes on two separate
  `http.ServeMux` instances, `public` and `protected`; `httpx.RequireSession`
  wraps only the `protected` mux. Feature modules opt into each mux via
  `RegisterPublicRoutes`/`RegisterProtectedRoutes`. A public wildcard segment
  can shadow a protected literal path with the same prefix (Go's `ServeMux`
  resolves precedence per-mux, so `protected`'s more specific pattern is
  invisible to `public`); `GET /users/me` and `GET /users/suggested` are
  registered directly on `public`, individually wrapped with
  `httpx.RequireSession`, to win against the `GET /users/{username}` wildcard.
- Public routes shared with signed-in viewers (e.g. `GET /posts/{publicId}`)
  are wrapped with `httpx.OptionalSession` instead of `RequireSession`: it
  populates the viewer id in context when a valid session cookie is present
  but never rejects the request otherwise, so a handler's optional
  `httpx.UserID(r)` read reflects the real viewer (correct `liked`,
  `isFollowing`, and email visibility) instead of always looking anonymous.
- Feature modules: `users`, `sessions`, `posts`, `comments`, `uploads`,
  `search`, `notifications`, `feed`.
- Each feature module owns its PostgreSQL repository implementation in a
  co-located `database.go`; shared PostgreSQL lifecycle, SQL helpers, and
  database-specific resilience configuration live in `store/database`.
- Upload handler decodes and re-encodes images to JPEG, stripping EXIF/GPS
  metadata and enforcing a 25 MP pixel dimension limit before storage.
- `notifications-consumer` and `feed-consumer`: franz-go consumer groups, each
  subscribing to both `entity-changes` and `activity` topics with distinct
  consumer group names so each receives the full stream independently. Record
  handling recovers panics per record and continues the batch so a single
  malformed or buggy event cannot restart the whole poll loop indefinitely.
- The backend exposes `GET /health/background` with lightweight in-memory
  progress for the outbox relay, notifications consumer, and feed consumer
  when `REDPANDA_BROKERS` enables those pipelines. Readiness still pings
  PostgreSQL and also fails when a configured background pipeline exits or has
  not heartbeated within the configured stale window.
- Session cleanup goroutine: sweeps expired sessions and deletes `outbox` rows
  older than 7 days every hour.

### Database (migrate/migrate)

- Runs as init container in the backend pod before the backend starts.
- Applies migrations from `apps/database/migrations/` using paired up/down
  files.

## Key Integration Patterns

- **Outbox pattern**: every domain mutation writes a row to `outbox` inside the
  same transaction. Payloads are marshaled from typed Go structs with
  `encoding/json`, never assembled with string formatting. The backend relay
  locks unpublished rows with `FOR UPDATE SKIP LOCKED`, publishes each row to
  its topic (`entity-changes` or `activity`), and marks `published_at` only
  after Kafka accepts the message. If the mark step fails, a later poll may
  duplicate the message; downstream consumers are idempotent.
- **Derived sync**: Redpanda Connect streams mode runs the `sync-search`
  Kafka consumer pipeline (entity-changes → Meilisearch). A one-shot
  Kubernetes Job (`broker-backfill`) seeds existing users, posts, and hashtags
  on first deploy. Post blob deletion is handled by the backend's authenticated
  S3 client during the delete request.
- **Resilience primitives**: reusable retry and circuit-breaker primitives live
  in the backend's `internal/resilience` package. PostgreSQL operations go
  through `database.DB.Read`/`Write`, which configure those primitives with the
  database transient-error classifier (5 consecutive transient failures → open;
  30 s cooldown). Only read operations are retried; writes are not retried
  unless a caller can prove idempotence. PostgreSQL and Dragonfly clients set
  finite connection lifetimes so long-lived pods recycle dependency
  connections instead of keeping stale sockets indefinitely.
- **Token bucket rate limiting**: implemented in Lua on Dragonfly; keyed by
  `{policy}:user:{id}` > `{policy}:session:{id}` > `{policy}:ip:{ip}`.
- **Login throttle**: per-IP (5 failures) and per-email (50 failures) counters
  stored in Dragonfly with 15 min TTL.
- **Cursor pagination**: all list endpoints use `(created DESC, id DESC)`
  composite cursors encoded as base64 JSON.
