# Business Rules

## User Registration

- Name: required, trimmed.
- Username: 3–30 chars, `^[a-z0-9._]+$`, normalized to lowercase. Unique across
  all users.
- Email: matches `^[^@\s]+@[^@\s]+\.[^@\s]+$`, normalized to lowercase. Unique
  across all users.
- Password: 8–1024 bytes.
- Duplicate username or email returns 409 Conflict.

## Profile Updates

- Name and username required.
- Username and email must satisfy same format rules as registration.
- Bio: max 300 Unicode code points.
- Avatar update: the provided filename must be a pending upload owned by the
  user, or the user's current avatar, or a filename already used in one of the
  user's posts. Any other value rolls back the transaction.
- Old avatar blob is deleted from object storage if no post or user still
  references it.

## Password Change

- Both current password and new password are required.
- Current password is verified with constant-time Argon2id comparison before
  accepting the new hash.
- All sessions other than the current one are deleted atomically in the same
  transaction.

## Image Upload

- One pending upload per user per call. Expired uploads (rows older than 1 hour)
  are deleted before a new one is created.
- Backend enforces 1 MB multipart limit via `io.LimitReader`.
- Frontend targets < 900 KB by resizing (canvas JPEG re-encode, quality steps
  from 0.88 to 0.60, scale steps of 0.85×, min dimension 320 px).
- Accepted formats: JPEG, PNG, GIF, WEBP (magic byte validation).

## Post Creation

- The provided filename must reference a pending upload owned by the current
  user. The upload row is deleted atomically in the same transaction.
- Description: optional, max 1000 Unicode code points.
- Hashtags are extracted with `#([A-Za-z0-9_]{1,50})`, lowercased, de-duplicated
  in first-occurrence order, and stored in `hashtags` and `post_hashtags`.
- Hashtag search-sync events are emitted after the `post_hashtags` row is
  written and carry the resulting visible `post_count`.

## Post Deletion

- Only the post's owner can delete it.
- Deleting a post clears `users.avatar` for any user whose avatar matches the
  post's filename.
- The associated blob is deleted from object storage after the database
  transaction commits.

## Comments

- Body: required, trimmed, 1–400 Unicode code points.
- A comment may be deleted by its author or by the owner of the post it belongs
  to.
- `ListComments` returns 404 if the post does not exist.

## Likes

- `LikePost` checks post existence first; returns 404 if not found.
- A user may like their own post.
- `ON CONFLICT DO NOTHING` makes liking idempotent.
- Unliking a non-liked post is a no-op (no error).

## Feed Logic

The feed reads from a pre-materialized `feed` table populated by the
`feed-consumer`:

- **Post created**: one feed row is inserted per follower of the author, plus
  one for the author themselves. Uses
  `ON CONFLICT (user_id, post_id) DO NOTHING` for idempotency.
- **Post deleted**: handled entirely by `ON DELETE CASCADE` on `feed.post_id` —
  no consumer action.
- **Follow**: the last 50 posts by the followed user are backfilled into the
  follower's feed.
- **Unfollow**: all feed rows where the post's author is the unfollowed user are
  pruned from the follower's feed.
- A new user's feed is empty until they follow someone or create a post.

## Celebrity / Hybrid Fanout

- **Celebrity threshold**: users with more than 10 000 followers skip
  fan-out-on-write; only their own feed row is written at post creation time.
- **Permanent latch**: celebrity status (`is_celebrity`) is a one-way boolean on
  the `users` row, set when `follower_count` first crosses the threshold and
  never reset. Fan-out routing decisions (both write-path and read-path) use
  `is_celebrity`, not the volatile `follower_count`, so posts pushed before the
  threshold was crossed are never lost if a user later loses followers.
- **Hybrid read**: celebrity posts and the viewer's own posts are merged into
  the feed at query time via a single `UNION ALL` branch — both are read
  live, deduped with `NOT EXISTS` against the materialized `feed` table so no
  duplicate appears once fan-out catches up.
- **Own-post read-after-write**: fan-out is asynchronous (outbox relay →
  Kafka → `feed-consumer`), so a viewer's own just-created post may not have a
  `feed` row yet. It is covered by the same live-read branch as celebrity
  posts, so it's visible immediately regardless of fan-out completion.

## Notifications

Notifications are created and deleted by the `notifications-consumer` from the
`activity` topic. The `entity-changes` topic is also consumed for post-deletion
cleanup. Self-events are never notified.

**Creation** (all skip if `actor_id == recipient_id`):

- Like: notify the post owner (`type=like`, `entity_id=post_public_id`).
- Comment: notify the post owner (`type=comment`, `entity_id=comment_id`).
- Follow: notify the followee (`type=follow`, `entity_id=actor_id as string`).

All inserts use `ON CONFLICT (external_id) DO NOTHING` for idempotency.

**Deletion**:

- Unlike: delete the matching like notification for the actor and post.
- Comment deleted: delete the matching comment notification by
  `entity_id=comment_id`.
- Unfollow: delete the matching follow notification for the actor and recipient.
- Post deleted: delete all `like` and `comment` notifications where
  `entity_id=post_public_id`.

## Search Query Rules

- Query length: 1–50 UTF-8 rune count (validated before hitting Meilisearch or
  PostgreSQL).
- `GET /search?type=posts&q=#tag` filters by exact hashtag match; the tag is
  validated against `^[A-Za-z0-9_]{1,50}$`.
- `GET /search?type=posts&q=text` performs full-text search on description and
  username.
- Typeahead endpoints (`/users/search`, `/hashtags/search`) use PostgreSQL
  trigram similarity (`%` operator) when Meilisearch is absent; Meilisearch when
  present (up to 8 results).
- `GET /search?type=all` blends users, posts, and hashtags into one ranked
  page instead of three separate result sets (`computeBlendTargets`: roughly
  20/60/20 users/posts/hashtags per page, minimum 1 user and 1 hashtag once
  `limit >= 3`). Within a page, users the viewer follows are moved to the
  front of that page's user results (stable otherwise); this never reorders
  across pages. Each entity type's Meilisearch offset advances only by how
  many of its fetched results were actually placed on the page, so a result is
  never skipped or duplicated across pages — a type that comes up short is
  backfilled from another type's surplus within the same page, and a page can
  come back smaller than `limit` when two types are simultaneously scarce.

## Follow Rules

- A user cannot follow themselves (rejected at service layer).
- Follow is idempotent (`ON CONFLICT DO NOTHING`).
- Following a non-existent user returns 404 (verified by CTE before insert).

## Session Expiry Rules

- Sliding TTL: 7 days from last use (refreshed on each authenticated request if
  within the inner half of the window).
- Absolute TTL: 720 hours (30 days) from creation, configurable via
  `SESSION_ABSOLUTE_TTL_HOURS`.
- Sessions outside either window are rejected and the cookie is cleared.

## Pagination Rules

- `limit`: 1–50; values > 50 are clamped to 50; default 10.
- `page` parameter is explicitly rejected (400).
- Cursor validity: must be base64url-encoded JSON
  `{created: timestamp, id: positive int}`.
- Search endpoint cursor encodes a Meilisearch offset (base64 integer string)
  for `type=users|posts|hashtags`, or independent per-type offsets
  (base64 JSON `{u, p, h}`) for `type=all`.

## Ordering Guarantees

- Feed, user posts, liked posts, followers, following:
  `(relevant_timestamp DESC, id DESC)`.
- Comments: `(created DESC, id DESC)`.
- All orderings are stable because `id` is appended as a tiebreaker.
