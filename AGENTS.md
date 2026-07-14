# Phasma

## Architecture

Three services are deployed through Kubernetes:

- `apps/backend` — Go API using `net/http` and `pgx`
- `apps/database` — `migrate/migrate` migration image, run as a backend init
  container
- `apps/frontend` — SvelteKit 2 SSR application using Svelte 5, strict
  TypeScript, Vite, adapter-node, Tailwind v4, DaisyUI 5, and Lucide

Each service owns its dependencies and Dockerfile. There is no monorepo or npm
workspace tooling.

Before changing files under `apps/backend/` or `apps/frontend/`, read that
directory's `AGENTS.md`.

## Commands

```sh
make                     # build and push all images
make backend             # backend image
make database            # migration image
make frontend            # frontend image
make format              # format backend Go code
make lint                # backend formatting check and frontend lint
make test                # backend and frontend unit tests
./scripts/deploy.sh      # preferred local Kubernetes deployment
```

Images default to `localhost:5000/phasma/<service>` and can be redirected with
`REGISTRY` in `scripts/deploy.sh` or `IMAGE_PREFIX` in `make`.

### Manual Kubernetes workflow

```sh
kubectl create namespace phasma
kubectl create secret generic database-secret -n phasma \
  --from-literal=database-password="$(openssl rand -hex 32)"
kubectl create secret generic backend-secret -n phasma \
  --from-literal=session-hash-secret="$(openssl rand -hex 32)"
kubectl create secret generic storage-secret -n phasma \
  --from-literal=s3-access-key="$(openssl rand -hex 32)" \
  --from-literal=s3-secret-key="$(openssl rand -hex 32)"
kubectl create secret generic cache-secret -n phasma \
  --from-literal=cache-password="$(openssl rand -hex 32)"
kubectl create secret generic search-secret -n phasma \
  --from-literal=search-master-key="$(openssl rand -hex 32)"
kubectl create secret generic app-db-secret -n phasma \
  --from-literal=app-db-password="$(openssl rand -hex 32)"
kubectl create secret generic connect-secret -n phasma \
  --from-literal=connect-db-password="$(openssl rand -hex 32)" \
  --from-literal=search-connect-key=""  # filled by deploy.sh after Meilisearch is ready
kubectl apply -f ./deploy -n phasma
kubectl port-forward service/frontend 8080 -n phasma
```

Cleanup:

```sh
kubectl delete -f ./deploy -n phasma
kubectl delete namespace phasma
```

## Engineering Standards

- Follow SOLID, KISS, DRY, YAGNI, and the Pareto principle. Keep changes
  focused; do not build for hypothetical requirements.
- Search for an existing helper, abstraction, or platform primitive before
  adding one. Add abstractions only when they remove concrete complexity or
  duplication — a second occurrence already justifies extracting; don't
  extract before any duplication exists.
- Match surrounding structure, naming, and idioms so the codebase reads as one
  system.
- Use precise names and standard initialisms. Prefer clarity over compressed
  code and named constants over repeated policy-significant literals.
- Keep related fixes together; do not expand a task into unrelated cleanup.
- Comments explain constraints, invariants, security decisions, or non-obvious
  intent. Do not narrate straightforward code or preserve implementation
  history.
- Keep comments concise but valuable. Reduce verbose 3-5 line explanations to
  1-2 lines when the same constraint or intent remains clear, while preserving
  documentation comments and language requirements such as Go exported
  identifier docs.
- Do not suppress compiler, linter, type-checker, or test warnings to make
  checks pass. Fix the underlying issue. Use a narrowly scoped suppression only
  when required by an external API, generated code, or a documented false
  positive, and explain why it is safe.
- Write behavior-oriented tests for critical paths, complex logic, and risky
  failure modes. Do not chase coverage percentages.
- Services must remain stateless and correct with multiple replicas.

## Git and Commits

- Keep one logical change per commit. Tests required by a behavior change belong
  in the same commit.
- Use Conventional Commits (`type(scope): description`) in imperative present
  tense. Include a scope when it adds useful context.
- Commit messages are one line, at most 72 characters, with no body, trailers,
  or issue references.
- Review the staged diff before committing. Create commits only when the user
  explicitly requests them.

## Secure Engineering

Security controls are design constraints, not review-time additions.

- Validate untrusted data where it enters the system. Bound request bodies,
  multipart fields, stream reads, pagination, and collection sizes before
  parsing or allocation.
- Authentication and authorization default to deny. Never trust a
  client-supplied user ID; derive identity from the validated session and keep
  ownership checks next to the protected operation.
- Use parameterized SQL exclusively. Make check-then-act operations atomic with
  a transaction, row lock, or database constraint.
- Keep secrets out of code, committed configuration, URLs, browser storage,
  generated artifacts, and logs.
- Use established cryptographic primitives and constant-time comparisons for
  credentials, MACs, and tokens. Do not invent cryptographic protocols.
- Never render user-controlled HTML directly. Validate user-controlled URLs
  against an explicit scheme and origin policy.
- Log structured operational metadata without credentials, session values,
  request bodies, unnecessary personal data, or raw user-controlled text.
- Justify new dependencies by their maintenance, security, image-size, and
  runtime cost.
- Containers run non-root with a read-only root filesystem, all capabilities
  dropped, and `seccompProfile: RuntimeDefault`.

## Resilience

- Hard dependencies (database, required backends) must be available at startup —
  fail fast if they are not. Soft dependencies (cache, rate limiter, search)
  must not block startup; start degraded and self-heal.
- Self-heal soft dependencies: attempt a synchronous connection first, then on
  failure retry in the background with exponential backoff and log on entering
  degraded mode and on recovery. The client library owns reconnection once the
  initial connection succeeds.
- Prefer client primitives that survive reconnection over state computed at
  startup. Do not cache server-side state that is lost when a connection resets.

## Kubernetes Resources

- Set CPU requests on every container; omit CPU limits. CFS quota throttles at
  the limit even when the node has spare cycles, causing latency spikes.
- Always set memory requests and limits. Memory is not compressible — OOM kill
  is preferable to silently exhausting node memory.
- Pin third-party images to specific versions. Never use `:latest`.

## Database Migrations

- Migrations live in `apps/database/migrations/` as paired
  `NNNNNN_description.up.sql` and `.down.sql` files.
- Use two-space indentation.
- Applied migration history is append-only. Correct deployed schemas with a new
  migration rather than rewriting an existing one.
- Migrations run through the backend deployment's init container and must
  complete before the backend starts.

## Definition of Done

Before reporting a change complete:

1. Identify touched untrusted inputs, validation, and resource bounds.
2. Confirm authentication, authorization, and ownership checks.
3. Confirm concurrent or cross-replica operations are atomic and retried work is
   idempotent.
4. Confirm network calls have timeouts, bounded reads, and deliberate retries.
5. Add or update behavior-oriented tests for critical success and failure paths.
6. Review the complete diff for correctness, security, unnecessary complexity,
   duplication, stale comments, and unrelated changes.
7. Run relevant formatters, linters, tests, and builds in proportion to risk.
8. Report checks that could not run and remaining risk explicitly.

## Specs

- Before making any code changes, read the relevant spec in `docs/`
  (architecture, api, data-model, security, business-rules, design-system,
  frontend, or infrastructure).
- Update the relevant spec to reflect the change — new endpoints, schema
  changes, rule additions, security controls, or infrastructure modifications —
  before marking work complete.
