# Phasma Go Backend

The backend is a stateless Go HTTP service built with `net/http` and `pgx`. It
owns users, sessions, posts, comments, likes, and image upload metadata.

## Local Commands

```sh
go run ./cmd/api
go test ./...
go build ./cmd/api
```

Startup fails when PostgreSQL is unavailable or `SESSION_HASH_SECRET` is unset.

- `DATABASE_URL`
- `SESSION_HASH_SECRET`
- `IMAGE_DIR` (optional, defaults to `/tmp`)

## Navigation

- Composition and authentication boundaries: `internal/app/`
- HTTP helpers and middleware: `internal/httpx/`
- Feature modules:
  `internal/{users,sessions,uploads,posts,comments,search,notifications,feed}/`
- Feature-owned PostgreSQL repositories: each feature module's `database.go`
- Shared PostgreSQL lifecycle and resilience configuration:
  `internal/store/database/`
- Reusable retry and circuit-breaking primitives: `internal/resilience/`
- Entrypoint and process lifecycle: `cmd/api/main.go`

Each feature module owns its domain types, repository interface, PostgreSQL
repository implementation, application service, HTTP handler, and route
registration. Handlers parse transport data and map responses; services own
workflows and authorization distinctions.

## API Conventions

- JSON request bodies are limited to 100 KB.
- Multipart uploads are limited to 1 MB.
- Errors use JSON message envelopes.
- Authentication uses the `session` cookie.
- Persisted session IDs are HMAC-SHA256 digests using `SESSION_HASH_SECRET`.
