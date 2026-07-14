# Data Model

## Entities and Relationships

```
users ──< sessions
users ──< uploads
users ──< posts ──< likes
                 ──< comments
                 ──< post_hashtags >── hashtags
users ──< follows (follower → followee)
users ──< notifications (recipient ← actor events)
users ──< feed >── posts
outbox (transactional outbox for Kafka entity-changes and activity events)
```

## Entity Definitions

### users

| Field          | Type         | Constraints                                                                                                                            |
| -------------- | ------------ | -------------------------------------------------------------------------------------------------------------------------------------- |
| id             | serial PK    | auto-increment — internal only, never exposed in API                                                                                   |
| public_id      | uuid         | UNIQUE, NOT NULL, DEFAULT gen_random_uuid() — external identifier; exposed as `id` in API responses                                    |
| name           | varchar(255) | NOT NULL                                                                                                                               |
| username       | varchar(30)  | UNIQUE, NOT NULL, CHECK `^[a-z0-9._]{3,30}$`                                                                                           |
| email          | varchar(255) | UNIQUE, NOT NULL                                                                                                                       |
| password       | varchar(255) | NOT NULL — Argon2id PHC hash                                                                                                           |
| avatar         | varchar(255) | DEFAULT `''` — blob filename                                                                                                           |
| bio            | varchar(300) | DEFAULT `''`                                                                                                                           |
| follower_count | int          | NOT NULL DEFAULT 0 — atomically maintained by follow/unfollow mutations within their transactions                                      |
| post_count     | int          | NOT NULL DEFAULT 0 — atomically maintained by post creation/deletion within their transactions                                         |
| is_celebrity   | boolean      | NOT NULL DEFAULT false — permanent one-way latch set when `follower_count` first exceeds the celebrity threshold; never reset to false |
| created        | timestamptz  | DEFAULT now()                                                                                                                          |

### sessions

| Field      | Type               | Constraints                                                                     |
| ---------- | ------------------ | ------------------------------------------------------------------------------- |
| id         | varchar(255) PK    | Private HMAC-SHA256 hash of the raw session token; never exposed                |
| public_id  | uuid               | UNIQUE, NOT NULL, DEFAULT gen_random_uuid() — listing and revocation identifier |
| user_id    | integer FK → users | NOT NULL, ON DELETE CASCADE                                                     |
| created    | timestamptz        | NOT NULL                                                                        |
| expires_at | timestamptz        | NOT NULL                                                                        |

### uploads

Staging table for image blobs before they become a post or avatar. A row is
consumed (deleted) atomically when a post or avatar update references it.

| Field    | Type               | Constraints                 |
| -------- | ------------------ | --------------------------- |
| filename | varchar(255) PK    | 32-char lowercase hex       |
| user_id  | integer FK → users | NOT NULL, ON DELETE CASCADE |
| created  | timestamptz        | NOT NULL                    |

### posts

| Field         | Type               | Constraints                                                                                   |
| ------------- | ------------------ | --------------------------------------------------------------------------------------------- |
| id            | serial PK          |                                                                                               |
| public_id     | uuid UNIQUE        | DEFAULT gen_random_uuid() — external identifier                                               |
| user_id       | integer FK → users | NOT NULL, ON DELETE CASCADE                                                                   |
| filename      | varchar(255)       | NOT NULL — blob filename                                                                      |
| description   | varchar(1000)      | nullable                                                                                      |
| like_count    | int                | NOT NULL DEFAULT 0 — atomically maintained by like/unlike mutations within their transactions |
| comment_count | int                | NOT NULL DEFAULT 0 — atomically maintained by comment create/delete within their transactions |
| created       | timestamptz        | NOT NULL                                                                                      |

### likes

| Field   | Type                          | Constraints                 |
| ------- | ----------------------------- | --------------------------- |
| post_id | integer FK → posts            | NOT NULL, ON DELETE CASCADE |
| user_id | integer FK → users            | NOT NULL, ON DELETE CASCADE |
| created | timestamptz                   | NOT NULL                    |
| —       | PRIMARY KEY(post_id, user_id) |                             |

### comments

| Field     | Type               | Constraints                                                       |
| --------- | ------------------ | ----------------------------------------------------------------- |
| id        | serial PK          |                                                                   |
| public_id | uuid               | UNIQUE, NOT NULL, DEFAULT gen_random_uuid() — external identifier |
| post_id   | integer FK → posts | ON DELETE CASCADE                                                 |
| user_id   | integer FK → users | ON DELETE CASCADE                                                 |
| body      | varchar(400)       | CHECK char_length > 0                                             |
| created   | timestamptz        | NOT NULL                                                          |

### hashtags

| Field      | Type               | Constraints                                                          |
| ---------- | ------------------ | -------------------------------------------------------------------- |
| id         | serial PK          |                                                                      |
| name       | varchar(50) UNIQUE | NOT NULL — lowercase                                                 |
| post_count | int                | NOT NULL DEFAULT 0 — atomically maintained by post creation/deletion |
| created    | timestamptz        | NOT NULL                                                             |

### post_hashtags

| Field      | Type                             | Constraints                 |
| ---------- | -------------------------------- | --------------------------- |
| post_id    | integer FK → posts               | NOT NULL, ON DELETE CASCADE |
| hashtag_id | integer FK → hashtags            | NOT NULL, ON DELETE CASCADE |
| created    | timestamptz                      | NOT NULL                    |
| —          | PRIMARY KEY(post_id, hashtag_id) |                             |

### follows

| Field       | Type                                  | Constraints                      |
| ----------- | ------------------------------------- | -------------------------------- |
| follower_id | integer FK → users                    | NOT NULL, ON DELETE CASCADE      |
| followee_id | integer FK → users                    | NOT NULL, ON DELETE CASCADE      |
| created     | timestamptz                           | NOT NULL                         |
| —           | PRIMARY KEY(follower_id, followee_id) |                                  |
| —           | CHECK `follower_id != followee_id`    | enforced in DB and service layer |

### notifications

| Field       | Type                | Constraints                                                                                         |
| ----------- | ------------------- | --------------------------------------------------------------------------------------------------- |
| id          | bigserial PK        | auto-increment                                                                                      |
| public_id   | uuid                | UNIQUE, NOT NULL, DEFAULT gen_random_uuid() — external identifier, exposed as `id` in API responses |
| external_id | varchar(255) UNIQUE | NOT NULL — idempotency key (outbox row id carried through Kafka)                                    |
| user_id     | bigint FK → users   | NOT NULL, ON DELETE CASCADE — recipient                                                             |
| actor_id    | bigint FK → users   | NOT NULL, ON DELETE CASCADE — who triggered the event                                               |
| type        | varchar(20)         | NOT NULL — CHECK `type IN ('like', 'comment', 'follow')`                                            |
| entity_id   | varchar(255)        | NOT NULL — post public_id, comment id, or actor user id                                             |
| read        | boolean             | NOT NULL DEFAULT false                                                                              |
| created     | timestamptz         | NOT NULL DEFAULT now()                                                                              |

### outbox

Transactional outbox for Redpanda. Written in the same transaction as the entity
mutation; the backend relay polls unpublished rows and publishes to the
appropriate topic. Payloads are generated with `encoding/json` from typed
backend structs so control characters and quotes are encoded as valid JSON. The
relay sets `published_at` only after Kafka accepts a row; a periodic cleanup
removes published rows older than 7 days.

| Field        | Type         | Constraints                               |
| ------------ | ------------ | ----------------------------------------- |
| id           | bigserial PK |                                           |
| topic        | varchar(50)  | NOT NULL — `entity-changes` or `activity` |
| payload      | jsonb        | NOT NULL — event payload                  |
| published_at | timestamptz  | NULL until the relay publishes the row    |
| created      | timestamptz  | NOT NULL DEFAULT now()                    |

### feed

Pre-materialized feed table. Populated by the `feed-consumer` on post creation
(fan-out on write) and follow events. `ON DELETE CASCADE` on both FKs handles
cleanup automatically when a post or user is deleted.

| Field   | Type                           | Constraints                 |
| ------- | ------------------------------ | --------------------------- |
| user_id | bigint FK → users              | NOT NULL, ON DELETE CASCADE |
| post_id | bigint FK → posts              | NOT NULL, ON DELETE CASCADE |
| created | timestamptz                    | NOT NULL                    |
| —       | PRIMARY KEY (user_id, post_id) |                             |

### audit_log

Append-only record of security-relevant account actions (e.g. password
change).

| Field   | Type              | Constraints                 |
| ------- | ----------------- | --------------------------- |
| id      | bigserial PK      |                             |
| user_id | bigint FK → users | NOT NULL, ON DELETE CASCADE |
| action  | varchar(50)       | NOT NULL                    |
| created | timestamptz       | NOT NULL DEFAULT now()      |

## Database Roles

| Role             | Privileges                                                                                                                             |
| ---------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| `phasma_app`     | SELECT/INSERT/UPDATE/DELETE on all tables, USAGE/SELECT on all sequences — used by the backend                                         |
| `phasma_connect` | SELECT only on `outbox`, `users`, `posts`, `hashtags`, `post_hashtags` — used by Kafka Connect to sync entity changes into Meilisearch |

## Indexes

| Table         | Index                                                          | Purpose                              |
| ------------- | -------------------------------------------------------------- | ------------------------------------ |
| sessions      | sessions_public_id_key (UNIQUE)                                | public session lookup and revocation |
| sessions      | sessions_user_id_idx                                           | session lookup by user               |
| sessions      | sessions_expires_at_idx                                        | cleanup sweep                        |
| uploads       | uploads_user_id_idx, uploads_created_idx                       | expiry queries                       |
| posts         | posts_created_id_idx                                           | feed pagination                      |
| posts         | posts_user_id_created_id_idx                                   | profile pagination                   |
| likes         | likes_user_id_created_post_id_idx                              | liked posts pagination               |
| comments      | comments_post_id_created_id_idx                                | comment pagination                   |
| comments      | comments_public_id_idx (UNIQUE)                                | public comment lookup and deletion   |
| follows       | follows_follower_id_idx, follows_followee_id_idx               | social graph traversal               |
| hashtags      | hashtags_name_trgm_idx (GIN trgm)                              | trigram typeahead                    |
| post_hashtags | post_hashtags_hashtag_id_idx                                   | hashtag → post lookup                |
| users         | users_username_trgm_idx (GIN trgm)                             | trigram typeahead                    |
| users         | users_is_celebrity_idx (partial, WHERE is_celebrity = true)    | celebrity fan-out routing            |
| outbox        | outbox_created_idx                                             | TTL cleanup                          |
| outbox        | outbox_published_at_idx (partial, WHERE published_at IS NULL)  | relay polling for unpublished rows   |
| notifications | notifications_public_id_idx (UNIQUE)                           | public notification lookup           |
| notifications | notifications_user_id_created_idx                              | keyset pagination per user           |
| notifications | notifications_type_entity_id_idx                               | bulk delete by type and entity       |
| notifications | notifications_user_id_unread_idx (partial, WHERE read = false) | fast unread count and list           |
| posts         | posts_filename_idx (partial, WHERE filename != '')             | orphan-upload checks                 |
| feed          | feed_user_id_created_idx                                       | keyset pagination per user           |
| audit_log     | audit_log_user_id_idx                                          | audit lookup by user                 |

## Meilisearch Indexes

| Index    | Searchable            | Filterable | Sortable   | Primary key                   | Stored (not searchable) |
| -------- | --------------------- | ---------- | ---------- | ----------------------------- | ----------------------- |
| users    | username, name        | —          | —          | id (string, stringified int)  | avatar                  |
| posts    | description, username | hashtags   | created    | post_id (post public_id UUID) | filename                |
| hashtags | name                  | —          | post_count | name                          | —                       |

`avatar` and `filename` are carried through the outbox → Kafka → Meilisearch
sync pipeline so search results can render a real avatar/thumbnail instead of
a placeholder; they are populated on each entity's next create/update and are
not backfilled for already-indexed rows.

## Domain Invariants

- A post's `filename` is consumed from `uploads` atomically at creation; an
  orphaned upload is never used for a post.
- A user's `avatar` must either be blank, equal to an existing `users.avatar` or
  `posts.filename` owned by the same user, or a valid pending `upload`. The old
  avatar blob is deleted from object storage if no other entity references it.
- Deleting a post clears `users.avatar` for any user whose avatar references the
  post's filename.
- A session token is stored only as its private HMAC-SHA256 `id`; the raw token
  lives only in the cookie. The independent UUID `public_id` is safe to expose
  for session listing and ownership-constrained revocation.
- Session creation is serialized on the owning user row and retains at most the
  100 newest sessions per user.
- Hashtag names are lowercased and de-duplicated before storage; they are
  created idempotently (`ON CONFLICT DO NOTHING`).
- `follows(follower_id, followee_id)` is inserted with `ON CONFLICT DO NOTHING`;
  self-follow is blocked at both the service layer and a database `CHECK`
  constraint.
- `feed(user_id, post_id)` inserts use `ON CONFLICT DO NOTHING`; post-deletion
  rows are removed by `ON DELETE CASCADE`, not by the consumer.
- `notifications` inserts use `ON CONFLICT (external_id) DO NOTHING`;
  `external_id` is the outbox row id relayed through Kafka, ensuring idempotent
  replay.
