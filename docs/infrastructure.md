# Infrastructure

## Kubernetes Deployment Model

All services are deployed to the `phasma` namespace via manifests in `deploy/`.
Local deployment targets a `kind` cluster.

| Workload   | Kind        | Replicas | Storage                                  |
| ---------- | ----------- | -------- | ---------------------------------------- |
| `frontend` | Deployment  | 2        | none (emptyDir /tmp)                     |
| `backend`  | Deployment  | 2        | none (stateless)                         |
| `database` | StatefulSet | 1        | PVC 5 Gi (ReadWriteOnce)                 |
| `storage`  | StatefulSet | 1        | PVC 5 Gi (ReadWriteOnce) + emptyDir /tmp |
| `cache`    | StatefulSet | 1        | PVC 1 Gi (ReadWriteOnce)                 |
| `search`   | StatefulSet | 1        | PVC 1 Gi (ReadWriteOnce) + emptyDir /tmp |
| `broker`   | StatefulSet | 1        | PVC 2 Gi (ReadWriteOnce)                 |
| `connect`  | Deployment  | 1        | none (stateless)                         |

## Image Registry

All custom images are pushed to `localhost:5000/phasma/<service>:<git-sha>`,
where the tag is the short Git commit SHA
(`GIT_SHA ?= $(shell git rev-parse --short HEAD)`). Built and pushed via
top-level `Makefile` targets; `scripts/deploy.sh` passes the SHA to `make` and
uses `kubectl set image` to roll out each new tag. Third-party images in
Kubernetes manifests are pinned to explicit version tags; do not use implicit
`latest`.

## Init Container Sequencing

The backend pod runs two init containers before the backend container starts:

1. `migration` — runs the `database` migration image as a non-root init
   container. Migrations must complete successfully before the next init
   container runs.
2. `provision-app-user` — connects as the `postgres` superuser and creates (or
   re-passwords) `phasma_user`, then grants it the `phasma_app` role. The
   backend container connects as `phasma_user`.

The broker-backfill Job runs one init container:

1. `provision-connect-user` — connects as the `postgres` superuser and creates
   (or re-passwords) `phasma_connect_user`, then grants it the `phasma_connect`
   role. The backfill container connects as `phasma_connect_user` with SELECT
   on `users`, `posts`, `hashtags`, and `post_hashtags`; the connect Deployment
   uses Kafka, Meilisearch, and S3 only.

`deploy.sh` provisions a Meilisearch scoped key (`documents.add` and
`documents.delete` on `users`, `posts`, `hashtags` indexes only) after
Meilisearch is ready and stores it in `connect-secret`. The connect Deployment
is restarted after this provisioning step to pick up the new key.

## Service Accounts

Each workload has a dedicated `ServiceAccount` defined in
`deploy/serviceaccounts.yaml` with `automountServiceAccountToken: false`.
Dedicated accounts allow per-workload RBAC if needed in future without granting
access at a shared default account. No RBAC rules are currently bound to any of
these accounts.

## Services and Networking

All services are cluster-internal only. The nginx Ingress exposes only the
`frontend` service on port 8080. NetworkPolicies default-deny both ingress and
egress in the namespace. Egress is re-opened only for kube-dns (all pods), and
for the specific pod-to-pod paths required by the service graph:
frontend→backend, backend→database/cache/search/storage/broker,
broker→database/search/storage/broker. Ingress is re-opened symmetrically.

| Service name | Port | Protocol         |
| ------------ | ---- | ---------------- |
| `frontend`   | 8080 | HTTP             |
| `backend`    | 8080 | HTTP             |
| `database`   | 5432 | PostgreSQL       |
| `storage`    | 8333 | S3 (HTTP)        |
| `cache`      | 6379 | Redis-compatible |
| `search`     | 7700 | HTTP             |
| `broker`     | 9092 | Kafka            |

`database`, `storage`, `cache`, `search`, and `broker` use headless services
(`clusterIP: None`). The `broker` service sets `publishNotReadyAddresses: true`.

## Ingress

nginx Ingress at `phasma.localhost`. Routes HTTPS traffic to `frontend:8080` and
redirects HTTP to HTTPS. `proxy-body-size: 2m` accommodates 1 MB image uploads
plus multipart overhead. Local deployment creates a self-signed `frontend-tls`
Secret; production should replace it with an ingress-managed certificate.

## Database TLS

All PostgreSQL connections use `sslmode=require`. The `database` StatefulSet is
configured with `-c ssl=on`, `-c ssl_cert_file=/certs/tls.crt`, and
`-c ssl_key_file=/certs/tls.key`. The cert and key are mounted from the
`database-tls` Secret at `/certs` with mode `0640` so that the postgres user
(uid/gid 70, `fsGroup: 70`) can read the key file. Local deployment generates a
self-signed `database-tls` Secret with CN `database` and SANs for the Kubernetes
service name; production should replace it with a properly issued certificate.

## Secrets

Secrets are split per service to limit blast radius:

| Secret            | Key                   | Consumer                                                                                                        |
| ----------------- | --------------------- | --------------------------------------------------------------------------------------------------------------- |
| `database-secret` | `database-password`   | PostgreSQL, migration init container, provision-app-user init container                                         |
| `app-db-secret`   | `app-db-password`     | provision-app-user init container, backend DATABASE_URL                                                         |
| `backend-secret`  | `session-hash-secret` | Backend HMAC session hashing                                                                                    |
| `storage-secret`  | `s3-access-key`       | Backend S3 client, SeaweedFS config                                                                             |
| `storage-secret`  | `s3-secret-key`       | Backend S3 client, SeaweedFS config                                                                             |
| `cache-secret`    | `cache-password`      | Backend cache client                                                                                            |
| `search-secret`   | `search-master-key`   | Backend search key provisioning, search service                                                                 |
| `connect-secret`  | `connect-db-password` | broker-backfill provision-connect-user init container and DATABASE_URL                                          |
| `connect-secret`  | `search-connect-key`  | connect Meilisearch scoped key (documents.add/delete only; provisioned by deploy.sh after Meilisearch is ready) |

## Security Context (all pods)

- `runAsNonRoot: true`
- `automountServiceAccountToken: false`
- `allowPrivilegeEscalation: false`
- `readOnlyRootFilesystem: true` (backend, frontend, cache, search, storage
  init, broker, connect; not PostgreSQL)
- `capabilities: drop: [ALL]`
- `seccompProfile: RuntimeDefault`

| Service                                   | UID                           |
| ----------------------------------------- | ----------------------------- |
| backend                                   | 65532                         |
| backend migration init container          | 65532                         |
| backend provision-app-user init container | root (default postgres image) |
| frontend                                  | 1000                          |
| database                                  | 70                            |
| cache                                     | 1000                          |
| search                                    | 1000                          |
| storage                                   | 65532                         |
| broker                                    | 101                           |
| connect                                   | 101                           |

## Resource Limits

| Service                                   | Memory limit | Memory request | CPU request |
| ----------------------------------------- | ------------ | -------------- | ----------- |
| frontend                                  | 768 Mi       | 256 Mi         | 250 m       |
| backend                                   | 256 Mi       | 128 Mi         | 100 m       |
| backend migration init container          | 64 Mi        | 32 Mi          | 50 m        |
| backend provision-app-user init container | 64 Mi        | 64 Mi          | 50 m        |
| database                                  | 512 Mi       | 512 Mi         | 500 m       |
| cache                                     | 384 Mi       | 256 Mi         | 100 m       |
| search                                    | 512 Mi       | 256 Mi         | 100 m       |
| storage                                   | 256 Mi       | 128 Mi         | 100 m       |
| broker                                    | 512 Mi       | 256 Mi         | 200 m       |
| connect                                   | 256 Mi       | 128 Mi         | 100 m       |

Dragonfly starts with `--proactor_threads=1` so its thread-derived memory floor
fits the single-node local resource envelope.

Redpanda starts through `rpk redpanda start` as a single broker with explicit
local sizing and addresses: `--config=/tmp/redpanda.yaml`, `--smp=1`,
`--mode=dev-container`, `--memory=256M`, `--reserve-memory=0M`,
`--reactor-backend=epoll`, `--kernel-page-cache=1`, `--aio-fsync=0`,
`--max-networking-io-control-blocks=128`, `--node-id=0`, Kafka bound on
`0.0.0.0:9092` and advertised as
`broker-0.broker:9092`, and RPC bound on `0.0.0.0:33145` and advertised as
`broker-0.broker:33145`.

On Colima, Redpanda also requires enough node-level Linux AIO capacity. The
deploy script checks the `colima` context and fails before rollout if
`fs.aio-max-nr` is too low. Raise it with:

```sh
colima ssh -- sudo sysctl -w fs.aio-max-nr=1048576
```

To persist it in the Colima VM:

```sh
colima ssh -- sudo sh -c 'echo fs.aio-max-nr=1048576 >/etc/sysctl.d/99-redpanda-aio.conf && sysctl --system'
```

## Health Probes

| Service  | Liveness                  | Readiness                     | Startup                           |
| -------- | ------------------------- | ----------------------------- | --------------------------------- |
| backend  | GET /health               | GET /ready (pings PostgreSQL) | GET /health, 30×2 s               |
| frontend | GET /health               | GET /health                   | GET /health, 30×2 s               |
| database | pg_isready                | pg_isready                    | pg_isready, 30×2 s                |
| cache    | tcpSocket :6379           | tcpSocket :6379               | tcpSocket :6379, 30×2 s           |
| search   | GET /health               | GET /health                   | GET /health, 30×2 s               |
| storage  | tcpSocket :8333           | tcpSocket :8333               | tcpSocket :8333, 30×2 s           |
| broker   | GET :9644/v1/status/ready | GET :9644/v1/status/ready     | GET :9644/v1/status/ready, 60×2 s |
| connect  | GET :4195/ready           | GET :4195/ready               | GET :4195/ready, 30×2 s           |

## Pod Disruption Budgets

`deploy/pdb.yaml` defines a `PodDisruptionBudget` for each workload. `database`,
`cache`, `broker`, `search`, and `storage` use `maxUnavailable: 0` because they
are single-replica stateful services; any voluntary disruption would take them
down. `backend` and `frontend` use `minAvailable: 1` because they run 2
replicas, which allows rolling updates while keeping one replica available. Pod
anti-affinity spreads replicas across nodes to reduce correlated failure risk.

## Migration Strategy

- Migrations live in `apps/database/migrations/` as paired
  `NNNNNN_description.{up,down}.sql`.
- Applied via `migrate/migrate` in the init container before each backend
  rollout.
- Migration history is append-only; deployed schema defects require corrective
  migrations.

## PostgreSQL Outbox Relay

The backend relay polls `outbox` rows where `published_at IS NULL` using
`FOR UPDATE SKIP LOCKED`, publishes each row to the row's `topic`, then marks
`published_at` after Kafka accepts the message. Marking after publish gives
at-least-once relay semantics: a failed mark can duplicate a later publish, but
consumers are idempotent. PostgreSQL still runs with `wal_level=logical` and
keeps the `outbox_relay` publication for compatibility with a future
WAL-backed relay.

## Environment Variables (backend)

| Variable                          | Source                           | Purpose                                                                                                     |
| --------------------------------- | -------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `DATABASE_URL`                    | constructed from `app-db-secret` | PostgreSQL connection as `phasma_user` (least-privilege)                                                    |
| `SESSION_HASH_SECRET`             | secret                           | HMAC key for session tokens                                                                                 |
| `S3_ENDPOINT`                     | literal                          | SeaweedFS endpoint                                                                                          |
| `S3_BUCKET`                       | literal                          | S3 bucket name                                                                                              |
| `S3_REGION`                       | literal                          | S3 region                                                                                                   |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | secrets                          | S3 credentials                                                                                              |
| `CACHE_URL`                       | literal                          | Cache connection                                                                                            |
| `CACHE_PASSWORD`                  | secret                           | Cache auth                                                                                                  |
| `TRUST_PROXY`                     | literal `"true"`                 | Honor valid X-Forwarded-* headers from the ingress, which must overwrite client-supplied forwarding headers |
| `MEILI_URL`                       | literal                          | Meilisearch endpoint                                                                                        |
| `MEILI_MASTER_KEY`                | secret                           | Meilisearch key provisioning                                                                                |
| `REDPANDA_BROKERS`                | literal                          | Kafka broker address for consumers                                                                          |
| `PORT`                            | literal `"8080"`                 | Listen port                                                                                                 |

## Environment Variables (frontend)

| Variable          | Source                 | Purpose                                                                       |
| ----------------- | ---------------------- | ----------------------------------------------------------------------------- |
| `BACKEND_URL`     | literal                | Backend service URL                                                           |
| `BODY_SIZE_LIMIT` | literal `"1100K"`      | Allows resized image uploads plus multipart overhead before action validation |
| `NODE_ENV`        | literal `"production"` | Enables production-mode SvelteKit/runtime security defaults                   |
| `PORT`            | literal `"8080"`       | Listen port                                                                   |
| `PROTOCOL_HEADER` | literal                | Proxy protocol header                                                         |
| `HOST_HEADER`     | literal                | Proxy host header                                                             |
| `ADDRESS_HEADER`  | literal                | Proxy client IP header                                                        |

The frontend production manifest does not hardcode `ORIGIN`; adapter-node
derives the request origin from `X-Forwarded-Proto` and `X-Forwarded-Host` set
by the ingress. `scripts/deploy.sh` sets a local `ORIGIN` override only for
direct service port-forwarding.
