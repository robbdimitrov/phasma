package posts

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"phasma/backend/internal/pagination"
	"phasma/backend/internal/store"
	"phasma/backend/internal/store/database"
)

// $1 is the viewer id, empty for anonymous callers; NULLIF avoids casting the
// empty string to bigint, which would error instead of yielding a false
// "liked" match.
const postColumns = `posts.id, posts.public_id, posts.user_id, u.username, u.name, u.avatar,
	posts.filename, posts.description,
	posts.like_count AS likes,
	EXISTS (SELECT 1 FROM likes
	WHERE post_id = posts.id AND likes.user_id = NULLIF($1, '')::bigint) AS liked,
	posts.comment_count AS comments,
	posts.created`

type PostRepository struct {
	db *database.DB
}

func NewPostRepository(client *database.Client) *PostRepository {
	return &PostRepository{db: client.DB()}
}

func (r *PostRepository) CreatePost(ctx context.Context, userID, filename string, description *string, tags []string) (string, bool, error) {
	var publicID string
	err := r.db.Write(ctx, func() error {
		tx, err := r.db.Pool().Begin(ctx)
		if err != nil {
			return err
		}
		defer database.Rollback(ctx, tx)
		var consumed string
		if err := tx.QueryRow(ctx, `DELETE FROM uploads WHERE user_id = $1 AND filename = $2
			RETURNING filename`, userID, filename).Scan(&consumed); err != nil {
			return err
		}
		var postID int
		var createdAt time.Time
		if err := tx.QueryRow(ctx, `INSERT INTO posts (user_id, filename, description)
			VALUES ($1, $2, $3) RETURNING id, public_id, created`, userID, filename, description).Scan(&postID, &publicID, &createdAt); err != nil {
			return err
		}
		var username string
		var isCelebrity bool
		if err := tx.QueryRow(ctx,
			`UPDATE users SET post_count = post_count + 1
			WHERE id = $1 RETURNING username, is_celebrity`,
			userID).Scan(&username, &isCelebrity); err != nil {
			return err // hard fail — wrong celebrity status means wrong fan-out decision
		}

		descStr := ""
		if description != nil {
			descStr = *description
		}

		hashtags := tags
		if hashtags == nil {
			hashtags = []string{}
		}

		postPayload, err := database.MarshalOutboxPayload(database.EntityPostUpsertPayload{
			Table:       "posts",
			Op:          "upsert",
			ID:          int64(postID),
			PostID:      publicID,
			AuthorID:    userID,
			Description: descStr,
			Username:    username,
			Hashtags:    hashtags,
			Created:     createdAt.UTC().Format(time.RFC3339Nano),
			IsCelebrity: isCelebrity,
		})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO outbox (topic, payload) VALUES ($1, $2)`, "entity-changes", postPayload); err != nil {
			return err
		}

		for _, tag := range tags {
			if _, err := tx.Exec(ctx,
				`INSERT INTO hashtags (name) VALUES ($1)
				ON CONFLICT (name) DO NOTHING`, tag); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO post_hashtags (post_id, hashtag_id)
				SELECT $1, id FROM hashtags WHERE name = $2 ON CONFLICT DO NOTHING`, postID, tag); err != nil {
				return err
			}
			var postCount int
			if err := tx.QueryRow(ctx,
				`UPDATE hashtags
				SET post_count = (
					SELECT count(*) FROM post_hashtags WHERE hashtag_id = hashtags.id
				)
				WHERE name = $1 RETURNING post_count`, tag).Scan(&postCount); err != nil {
				return err
			}
			hashtagPayload, err := database.MarshalOutboxPayload(database.EntityHashtagUpsertPayload{
				Table:     "hashtags",
				Op:        "upsert",
				Name:      tag,
				PostCount: postCount,
			})
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO outbox (topic, payload) VALUES ($1, $2)`, "entity-changes", hashtagPayload); err != nil {
				return err
			}
		}
		return tx.Commit(ctx)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return publicID, true, nil
}

func (r *PostRepository) GetPosts(ctx context.Context, username string, cursor *pagination.Cursor, limit int, currentUserID string) ([]Post, *pagination.Cursor, error) {
	hasCursor, cursorCreated, cursorID := database.CursorValues(cursor)
	return r.queryPostPageOrNotFound(ctx, `WITH urow AS (SELECT id FROM users WHERE username = $2),
page AS (
    SELECT `+postColumns+`, posts.created AS cursor_created
    FROM posts JOIN users u ON u.id = posts.user_id
    WHERE posts.user_id = (SELECT id FROM urow)
    AND (NOT $3 OR (posts.created, posts.id) < ($4, $5))
    ORDER BY posts.created DESC, posts.id DESC LIMIT $6
)
SELECT *, true AS user_exists FROM page
UNION ALL
SELECT NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,
       (SELECT id FROM urow) IS NOT NULL
WHERE NOT EXISTS (SELECT 1 FROM page)`,
		limit, currentUserID, username, hasCursor, cursorCreated, cursorID, limit+1)
}

func (r *PostRepository) GetLikedPosts(ctx context.Context, username string, cursor *pagination.Cursor, limit int, currentUserID string) ([]Post, *pagination.Cursor, error) {
	hasCursor, cursorCreated, cursorID := database.CursorValues(cursor)
	return r.queryPostPageOrNotFound(ctx, `WITH urow AS (SELECT id FROM users WHERE username = $2),
page AS (
    SELECT `+postColumns+`, likes.created AS cursor_created
    FROM posts JOIN users u ON u.id = posts.user_id
    INNER JOIN likes ON likes.post_id = posts.id
    WHERE likes.user_id = (SELECT id FROM urow)
    AND (NOT $3 OR (likes.created, posts.id) < ($4, $5))
    ORDER BY likes.created DESC, posts.id DESC LIMIT $6
)
SELECT *, true AS user_exists FROM page
UNION ALL
SELECT NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,
       (SELECT id FROM urow) IS NOT NULL
WHERE NOT EXISTS (SELECT 1 FROM page)`,
		limit, currentUserID, username, hasCursor, cursorCreated, cursorID, limit+1)
}

func (r *PostRepository) GetPost(ctx context.Context, postID, currentUserID string) (Post, bool, error) {
	result, err := r.queryPosts(ctx, `SELECT `+postColumns+`
		FROM posts JOIN users u ON u.id = posts.user_id
		WHERE posts.public_id = $2`, currentUserID, postID)
	if err != nil {
		return Post{}, false, err
	}
	if len(result) == 0 {
		return Post{}, false, nil
	}
	return result[0], true, nil
}

func (r *PostRepository) DeletePost(ctx context.Context, postID, userID string) (string, error) {
	var filename string
	err := r.db.Write(ctx, func() error {
		tx, err := r.db.Pool().Begin(ctx)
		if err != nil {
			return err
		}
		defer database.Rollback(ctx, tx)

		// Lock the row to prevent concurrent deletes from racing on ownership.
		var postDBID int
		var ownerID string
		if err := tx.QueryRow(ctx,
			`SELECT id, user_id::text FROM posts WHERE public_id = $1 FOR UPDATE`,
			postID).Scan(&postDBID, &ownerID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return store.ErrNotFound
			}
			return err
		}
		if ownerID != userID {
			return store.ErrForbidden
		}

		// Look up hashtag names before deleting.
		hashtagRows, err := tx.Query(ctx,
			`SELECT h.name FROM hashtags h
			JOIN post_hashtags ph ON ph.hashtag_id = h.id
			WHERE ph.post_id = $1`, postDBID)
		if err != nil {
			return err
		}
		var hashtags []string
		for hashtagRows.Next() {
			var name string
			if err := hashtagRows.Scan(&name); err != nil {
				hashtagRows.Close()
				return err
			}
			hashtags = append(hashtags, name)
		}
		hashtagRows.Close()
		if err := hashtagRows.Err(); err != nil {
			return err
		}

		// Collect comment public IDs before deletion for notification cleanup.
		commentRows, err := tx.Query(ctx, `SELECT public_id::text FROM comments WHERE post_id = $1`, postDBID)
		if err != nil {
			return err
		}
		commentPublicIDs := []string{}
		for commentRows.Next() {
			var cid string
			if err := commentRows.Scan(&cid); err != nil {
				commentRows.Close()
				return err
			}
			commentPublicIDs = append(commentPublicIDs, cid)
		}
		commentRows.Close()
		if err := commentRows.Err(); err != nil {
			return err
		}

		if err := tx.QueryRow(ctx, `DELETE FROM posts WHERE id = $1 RETURNING filename`,
			postDBID).Scan(&filename); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET avatar = $1 WHERE avatar = $2`, "", filename); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE users SET post_count = GREATEST(post_count - 1, 0) WHERE id = $1`, userID); err != nil {
			return err
		}

		postPayload, err := database.MarshalOutboxPayload(database.EntityPostDeletePayload{
			Table:            "posts",
			Op:               "delete",
			ID:               int64(postDBID),
			PostID:           postID,
			AuthorID:         userID,
			Filename:         filename,
			CommentPublicIDs: commentPublicIDs,
		})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO outbox (topic, payload) VALUES ($1, $2)`, "entity-changes", postPayload); err != nil {
			return err
		}

		for _, tag := range hashtags {
			var postCount int
			if err := tx.QueryRow(ctx,
				`UPDATE hashtags SET post_count = GREATEST(post_count - 1, 0)
				WHERE name = $1 RETURNING post_count`, tag).Scan(&postCount); err != nil {
				return err
			}
			hashtagPayload, err := database.MarshalOutboxPayload(database.EntityHashtagUpsertPayload{
				Table:     "hashtags",
				Op:        "upsert",
				Name:      tag,
				PostCount: postCount,
			})
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO outbox (topic, payload) VALUES ($1, $2)`, "entity-changes", hashtagPayload); err != nil {
				return err
			}
		}

		return tx.Commit(ctx)
	})
	return filename, err
}

// likeMutation pairs a like/unlike SQL statement with the outbox op name and
// like_count delta it implies, so the three can't drift apart at a call site.
// sql must remain a hardcoded literal taking ($1=userID, $2=postID) and affect
// at most one row — toggleLike's not-found/no-op detection assumes that.
type likeMutation struct {
	op    string
	sql   string
	delta int
}

var likeInsert = likeMutation{
	op: "like",
	sql: `INSERT INTO likes (user_id, post_id)
		SELECT $1, id FROM posts WHERE public_id = $2
		ON CONFLICT DO NOTHING`,
	delta: 1,
}

var unlikeDelete = likeMutation{
	op: "unlike",
	sql: `DELETE FROM likes
		WHERE user_id = $1 AND post_id = (SELECT id FROM posts WHERE public_id = $2)`,
	delta: -1,
}

func (r *PostRepository) toggleLike(ctx context.Context, postID, userID string, m likeMutation) error {
	err := r.db.Write(ctx, func() error {
		tx, err := r.db.Pool().Begin(ctx)
		if err != nil {
			return err
		}
		defer database.Rollback(ctx, tx)

		tag, err := tx.Exec(ctx, m.sql, userID, postID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			// Distinguish "post gone" from "already in that state" atomically.
			var exists bool
			if err := tx.QueryRow(ctx,
				`SELECT EXISTS (SELECT 1 FROM posts WHERE public_id = $1)`, postID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return store.ErrNotFound
			}
			return tx.Commit(ctx)
		}

		if _, err := tx.Exec(ctx,
			`UPDATE posts SET like_count = GREATEST(like_count + $2, 0) WHERE public_id = $1`, postID, m.delta); err != nil {
			return err
		}

		var recipientID string
		if err := tx.QueryRow(ctx,
			`SELECT user_id::text FROM posts WHERE public_id = $1`, postID).Scan(&recipientID); err != nil {
			return err
		}

		payload, err := database.MarshalOutboxPayload(database.ActivityPayload{
			Op:          m.op,
			PostID:      postID,
			ActorID:     userID,
			RecipientID: recipientID,
		})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO outbox (topic, payload) VALUES ($1, $2)`, "activity", payload); err != nil {
			return err
		}

		return tx.Commit(ctx)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ErrNotFound
	}
	return err
}

func (r *PostRepository) LikePost(ctx context.Context, postID, userID string) error {
	return r.toggleLike(ctx, postID, userID, likeInsert)
}

func (r *PostRepository) UnlikePost(ctx context.Context, postID, userID string) error {
	return r.toggleLike(ctx, postID, userID, unlikeDelete)
}

func (r *PostRepository) queryPosts(ctx context.Context, query string, args ...any) ([]Post, error) {
	var result []Post
	err := r.db.Read(ctx, func() error {
		rows, err := r.db.Pool().Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		result = []Post{}
		for rows.Next() {
			var post Post
			var description, avatar sql.NullString
			if err := rows.Scan(&post.ID, &post.PublicID, &post.UserID, &post.Username,
				&post.Name, &avatar, &post.Filename, &description, &post.Likes,
				&post.Liked, &post.Comments, &post.Created); err != nil {
				return err
			}
			post.Avatar = database.NullableString(avatar)
			post.Description = database.NullableString(description)
			result = append(result, post)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// queryPostPageOrNotFound expects postColumns plus cursor_created and
// user_exists from the UNION ALL sentinel query.
func (r *PostRepository) queryPostPageOrNotFound(ctx context.Context, query string, limit int, args ...any) ([]Post, *pagination.Cursor, error) {
	type row struct {
		post          Post
		cursorCreated time.Time
	}
	var result []row
	err := r.db.Read(ctx, func() error {
		rows, err := r.db.Pool().Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		result = []row{}
		for rows.Next() {
			var id *int
			var publicID *string
			var userID *int
			var username, name *string
			var avatar, description sql.NullString
			var filename *string
			var likes *int
			var liked *bool
			var comments *int
			var created, cursorCreated *time.Time
			var userExists bool
			if err := rows.Scan(&id, &publicID, &userID, &username, &name, &avatar,
				&filename, &description, &likes, &liked, &comments, &created,
				&cursorCreated, &userExists); err != nil {
				return err
			}
			if id == nil {
				// Sentinel row: page is empty.
				if !userExists {
					return store.ErrNotFound
				}
				return nil
			}
			var item row
			item.post.ID = *id
			item.post.PublicID = *publicID
			item.post.UserID = *userID
			item.post.Username = *username
			item.post.Name = *name
			item.post.Avatar = database.NullableString(avatar)
			item.post.Filename = *filename
			item.post.Description = database.NullableString(description)
			item.post.Likes = *likes
			item.post.Liked = *liked
			item.post.Comments = *comments
			item.post.Created = *created
			item.cursorCreated = *cursorCreated
			result = append(result, item)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, nil, err
	}
	hasMore := len(result) > limit
	if hasMore {
		result = result[:limit]
	}
	items := make([]Post, len(result))
	for i, item := range result {
		items[i] = item.post
	}
	if !hasMore {
		return items, nil, nil
	}
	last := result[len(result)-1]
	return items, &pagination.Cursor{Created: last.cursorCreated, ID: int64(last.post.ID)}, nil
}

func (r *PostRepository) ListPopularPosts(ctx context.Context, viewerID string, limit int) ([]Post, error) {
	return r.queryPosts(ctx, `SELECT `+postColumns+`
        FROM posts
        JOIN users u ON u.id = posts.user_id
        WHERE posts.created > NOW() - INTERVAL '7 days'
          AND EXISTS (SELECT 1 FROM likes WHERE post_id = posts.id)
        ORDER BY likes DESC,
                 posts.created DESC
        LIMIT $2`, viewerID, limit)
}

var _ Repository = (*PostRepository)(nil)
