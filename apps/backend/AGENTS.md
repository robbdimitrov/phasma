# Backend Instructions

These rules extend the repository-level `AGENTS.md` for files under
`apps/backend/`.

## Stack and Navigation

- Go HTTP service using `net/http` and `pgx`
- Composition, routes, and authentication boundary: `internal/app/`
- HTTP helpers and middleware: `internal/httpx/`
- Feature modules:
  `internal/{users,sessions,uploads,posts,comments,search,notifications,feed}/`
- Feature-owned PostgreSQL repositories: each feature module's `database.go`
- Shared PostgreSQL lifecycle and resilience configuration:
  `internal/store/database/`
- Reusable retry and circuit-breaker primitives: `internal/resilience/`
- Process lifecycle: `cmd/api/main.go`

Each feature module owns its domain types, repository interface, PostgreSQL
repository implementation, service, handler, and route registration. Handlers
parse transport data and map responses. Services own workflows and
authorization distinctions.

## Commands

Run from `apps/backend/`:

```sh
go run ./cmd/api
go test ./...
go build ./cmd/api
```

Run `make test-integration` from the repository root for PostgreSQL repository
tests.

Missing or unreachable PostgreSQL fails startup.

## Go and API Conventions

- Keep handwritten Go `gofmt`-clean and use standard initialisms: `ID`, `URL`,
  `HTTP`, `DB`.
- Use symbolic `http.Status*` constants. HTTP APIs return JSON consistently,
  including errors.
- Parse request JSON through `httpx.DecodeJSON`; request bodies are bounded to
  100 KB.
- Bound multipart and stream reads before parsing. Uploads have a hard 1 MB
  multipart limit and must be content-sniffed from magic bytes rather than
  trusted extensions or MIME headers.
- Validate identifiers with established validators such as
  `validation.ValidUUID` and `ValidUsername`.
- Use typed fakes and `httptest` for behavior-oriented tests.

## Authentication and Authorization

- Authentication uses the `session` cookie, not JWT.
- Routes are registered on one of two `http.ServeMux` instances in
  `internal/app/routes.go`: `public` or `protected`. `httpx.RequireSession`
  wraps only `protected`. New public routes require justification. Because Go's
  `ServeMux` resolves precedence per-mux, a public wildcard segment (e.g.
  `GET /users/{username}`) can shadow a protected literal path with the same
  prefix; such literals must be registered directly on `public`, individually
  wrapped with `httpx.RequireSession`, instead of relying on `protected`.
- Never trust a client-supplied ID for authorization. Enforce ownership in the
  protected query or transaction where practical.
- Session IDs are generated with `crypto/rand`, validated, server-issued,
  HMAC-SHA256-hashed before persistence, and managed with a sliding TTL.
- Cookies remain `HttpOnly`, `SameSite=Strict`, and `Secure` behind TLS.
- Passwords use Argon2id PHC hashes with constant-time verification. Password
  changes revoke other sessions, and login failures remain throttled through
  per-IP and per-email counters in Dragonfly (`sessions.LoginThrottle`).

## PostgreSQL and Distributed State

- Use `pgx` parameterized queries exclusively.
- Any interpolated SQL fragment must come from a constrained internal constant
  and include a comment explaining the invariant.
- Make check-then-act operations atomic with a transaction, row lock, or
  database constraint.
- Do not retry non-idempotent writes. Operations eligible for retry must be
  demonstrably idempotent.
- The database-backed token bucket is the shared rate limiter. Add a
  `rateLimitPolicy` entry for new abuse-prone endpoints.
- An optional viewer id (present for a signed-in visitor, `""` for anonymous)
  compared against a `bigint` id column must be wrapped as
  `NULLIF($n, '')::bigint`, not compared bare — Postgres cannot cast `''` to
  `bigint`, and `id != NULLIF($n, '')::bigint` degrades correctly to `NULL`
  (excluding nothing) rather than `!=` against an empty string, which never
  matches. Every use of the shared parameter within the same query must use
  this same form, or Postgres infers conflicting types for the placeholder.

## Runtime and Operations

- `DATABASE_URL`, `S3_*`, and `SESSION_HASH_SECRET` are configured in
  `deploy/backend.yaml`. `PORT` defaults to `8080`.
- `TRUST_PROXY` is valid only behind a proxy that overwrites forwarding headers.
  Normalize trusted proxy-derived identity before rate limiting.
- Keep recovery middleware outermost.
- Background goroutines must recover panics at their boundary. Work
  intentionally outliving a request must not retain the request's cancellable
  context.
- Scheduled or singleton work spanning replicas requires durable distributed
  coordination.
- Use structured `slog` JSON with propagated request IDs. Log security-relevant
  decisions using safe operational metadata only.
- Health endpoints remain lightweight and exempt from authentication and rate
  limiting. Readiness checks must cover dependencies required to serve traffic.
