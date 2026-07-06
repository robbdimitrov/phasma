package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"phasma/backend/internal/feed"
	"phasma/backend/internal/pagination"
	"phasma/backend/internal/store"
	"phasma/backend/internal/store/database"
)

// $1 is the viewer id, empty for anonymous callers; NULLIF avoids casting the
// empty string to bigint, which would error instead of yielding a false
// "is_following" match.
const userColumns = `u.id, u.name, u.username, u.email, u.avatar, u.bio,
	u.post_count AS posts,
	(SELECT count(*) FROM likes WHERE user_id = u.id) AS likes,
	u.follower_count AS followers,
	(SELECT count(*) FROM follows WHERE follower_id = u.id) AS following,
	EXISTS (SELECT 1 FROM follows WHERE follower_id = NULLIF($1, '')::bigint AND followee_id = u.id) AS is_following,
	u.created,
	u.public_id::text`

type UserRepository struct {
	db *database.DB
}

func NewUserRepository(client *database.Client) *UserRepository {
	return &UserRepository{db: client.DB()}
}

func (r *UserRepository) CreateUser(ctx context.Context, name, username, email, passwordHash string) (int, error) {
	var id int
	err := r.db.Write(ctx, func() error {
		tx, err := r.db.Pool().Begin(ctx)
		if err != nil {
			return err
		}
		defer database.Rollback(ctx, tx)
		if err := tx.QueryRow(ctx, `INSERT INTO users (name, username, email, password)
			VALUES ($1, $2, $3, $4) RETURNING id`, name, username, email, passwordHash).Scan(&id); err != nil {
			return err
		}
		payload, err := database.MarshalOutboxPayload(database.EntityUserUpsertPayload{
			Table:    "users",
			Op:       "upsert",
			ID:       int64(id),
			UserID:   strconv.Itoa(id),
			Name:     name,
			Username: username,
			Bio:      "",
		})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO outbox (topic, payload) VALUES ($1, $2)`, "entity-changes", payload); err != nil {
			return err
		}
		return tx.Commit(ctx)
	})
	if err != nil {
		if database.UniqueViolation(err) {
			return 0, store.ErrConflict
		}
		return 0, err
	}
	return id, nil
}

func (r *UserRepository) GetUserWithID(ctx context.Context, userID string) (UserCredentials, bool, error) {
	var user UserCredentials
	err := r.db.Read(ctx, func() error {
		return r.db.Pool().QueryRow(ctx, `SELECT id, password FROM users WHERE id = $1`, userID).
			Scan(&user.ID, &user.PasswordHash)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return UserCredentials{}, false, nil
	}
	if err != nil {
		return UserCredentials{}, false, err
	}
	return user, true, nil
}

func (r *UserRepository) GetUserByUsername(ctx context.Context, username, currentUserID string) (User, bool, error) {
	return r.getUser(ctx, "username", username, currentUserID)
}

func (r *UserRepository) GetUserByID(ctx context.Context, userID, currentUserID string) (User, bool, error) {
	return r.getUser(ctx, "id", userID, currentUserID)
}

func (r *UserRepository) ListFollowers(ctx context.Context, username string, cursor *pagination.Cursor, limit int, currentUserID string) ([]User, *pagination.Cursor, error) {
	hasCursor, cursorCreated, cursorID := database.CursorValues(cursor)
	return r.queryUserPageOrNotFound(ctx, `WITH urow AS (SELECT id FROM users WHERE username = $2),
page AS (
    SELECT `+userColumns+`, f.created AS cursor_created
    FROM follows f
    JOIN users target ON target.id = f.followee_id
    JOIN users u ON u.id = f.follower_id
    WHERE target.id = (SELECT id FROM urow)
    AND (NOT $3 OR (f.created, u.id) < ($4, $5))
    ORDER BY f.created DESC, u.id DESC LIMIT $6
)
SELECT *, true AS user_exists FROM page
UNION ALL
SELECT NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,
       (SELECT id FROM urow) IS NOT NULL
WHERE NOT EXISTS (SELECT 1 FROM page)`,
		limit, currentUserID, username, hasCursor, cursorCreated, cursorID, limit+1)
}

func (r *UserRepository) ListFollowing(ctx context.Context, username string, cursor *pagination.Cursor, limit int, currentUserID string) ([]User, *pagination.Cursor, error) {
	hasCursor, cursorCreated, cursorID := database.CursorValues(cursor)
	return r.queryUserPageOrNotFound(ctx, `WITH urow AS (SELECT id FROM users WHERE username = $2),
page AS (
    SELECT `+userColumns+`, f.created AS cursor_created
    FROM follows f
    JOIN users target ON target.id = f.follower_id
    JOIN users u ON u.id = f.followee_id
    WHERE target.id = (SELECT id FROM urow)
    AND (NOT $3 OR (f.created, u.id) < ($4, $5))
    ORDER BY f.created DESC, u.id DESC LIMIT $6
)
SELECT *, true AS user_exists FROM page
UNION ALL
SELECT NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,
       (SELECT id FROM urow) IS NOT NULL
WHERE NOT EXISTS (SELECT 1 FROM page)`,
		limit, currentUserID, username, hasCursor, cursorCreated, cursorID, limit+1)
}

func (r *UserRepository) getUser(ctx context.Context, column, value, currentUserID string) (User, bool, error) {
	var user User
	var avatar, bio sql.NullString
	// column is an internal constant ("id" or "username"), never user input,
	// so interpolating it here is safe; values stay parameterized.
	err := r.db.Read(ctx, func() error {
		return r.db.Pool().QueryRow(ctx, `SELECT `+userColumns+`
			FROM users u WHERE u.`+column+` = $2`, currentUserID, value).Scan(
			&user.ID, &user.Name, &user.Username, &user.Email, &avatar, &bio,
			&user.Posts, &user.Likes, &user.Followers, &user.Following,
			&user.IsFollowing, &user.Created, &user.PublicID,
		)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	user.Avatar = database.NullableString(avatar)
	user.Bio = database.NullableString(bio)
	return user, true, nil
}

// queryUserPageOrNotFound expects userColumns plus cursor_created and
// user_exists from the UNION ALL sentinel query.
func (r *UserRepository) queryUserPageOrNotFound(ctx context.Context, query string, limit int, args ...any) ([]User, *pagination.Cursor, error) {
	type row struct {
		user          User
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
			var name, username, email *string
			var avatar, bio sql.NullString
			var userPosts, userLikes, followers, following *int
			var isFollowing *bool
			var created *time.Time
			var publicID *string
			var cursorCreated *time.Time
			var userExists bool
			if err := rows.Scan(&id, &name, &username, &email, &avatar, &bio,
				&userPosts, &userLikes, &followers, &following, &isFollowing,
				&created, &publicID, &cursorCreated, &userExists); err != nil {
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
			item.user.ID = *id
			item.user.PublicID = *publicID
			item.user.Name = *name
			item.user.Username = *username
			item.user.Email = *email
			item.user.Avatar = database.NullableString(avatar)
			item.user.Bio = database.NullableString(bio)
			item.user.Posts = *userPosts
			item.user.Likes = *userLikes
			item.user.Followers = *followers
			item.user.Following = *following
			item.user.IsFollowing = *isFollowing
			item.user.Created = *created
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
	items := make([]User, len(result))
	for i, item := range result {
		items[i] = item.user
	}
	if !hasMore {
		return items, nil, nil
	}
	last := result[len(result)-1]
	return items, &pagination.Cursor{Created: last.cursorCreated, ID: int64(last.user.ID)}, nil
}

func (r *UserRepository) FollowUser(ctx context.Context, followerID, followeeID string) error {
	var targetExists bool
	err := r.db.Write(ctx, func() error {
		tx, err := r.db.Pool().Begin(ctx)
		if err != nil {
			return err
		}
		defer database.Rollback(ctx, tx)
		var followInserted bool
		if err := tx.QueryRow(ctx, `WITH target AS (
				SELECT id FROM users WHERE id = $2
			), inserted AS (
				INSERT INTO follows (follower_id, followee_id)
				SELECT $1, id FROM target ON CONFLICT DO NOTHING
				RETURNING 1
			)
			SELECT EXISTS (SELECT 1 FROM target), EXISTS (SELECT 1 FROM inserted)`,
			followerID, followeeID).Scan(&targetExists, &followInserted); err != nil {
			return err
		}
		if !targetExists || !followInserted {
			// Either the target user doesn't exist or already followed — no outbox event.
			return tx.Commit(ctx)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE users SET
				follower_count = follower_count + 1,
				is_celebrity = is_celebrity OR (follower_count + 1 > $2)
			WHERE id = $1`,
			followeeID, feed.CelebThreshold); err != nil {
			return err
		}
		payload, err := database.MarshalOutboxPayload(database.ActivityPayload{
			Op:          "follow",
			ActorID:     followerID,
			RecipientID: followeeID,
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
	if err != nil {
		return err
	}
	if !targetExists {
		return store.ErrNotFound
	}
	return nil
}

func (r *UserRepository) UnfollowUser(ctx context.Context, followerID, followeeID string) error {
	return r.db.Write(ctx, func() error {
		tx, err := r.db.Pool().Begin(ctx)
		if err != nil {
			return err
		}
		defer database.Rollback(ctx, tx)
		tag, err := tx.Exec(ctx,
			`DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2`, followerID, followeeID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() > 0 {
			if _, err := tx.Exec(ctx,
				`UPDATE users SET follower_count = GREATEST(follower_count - 1, 0) WHERE id = $1`,
				followeeID); err != nil {
				return err
			}
			payload, err := database.MarshalOutboxPayload(database.ActivityPayload{
				Op:          "unfollow",
				ActorID:     followerID,
				RecipientID: followeeID,
			})
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO outbox (topic, payload) VALUES ($1, $2)`, "activity", payload); err != nil {
				return err
			}
		}
		return tx.Commit(ctx)
	})
}

func (r *UserRepository) UpdateUser(ctx context.Context, userID, name, username, email, avatar string, bio *string) (UpdateUserResult, error) {
	var result UpdateUserResult
	err := r.db.Write(ctx, func() error {
		tx, err := r.db.Pool().Begin(ctx)
		if err != nil {
			return err
		}
		defer database.Rollback(ctx, tx)

		var oldAvatar sql.NullString
		if err := tx.QueryRow(ctx, `SELECT avatar FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&oldAvatar); err != nil {
			return err
		}
		if avatar != "" {
			var avatarExists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (
				  SELECT 1 FROM users WHERE id = $1 AND avatar = $2
				  UNION SELECT 1 FROM posts WHERE user_id = $1 AND filename = $2
				) AS exists`, userID, avatar).Scan(&avatarExists); err != nil {
				return err
			}
			if !avatarExists {
				var consumed string
				if err := tx.QueryRow(ctx, `DELETE FROM uploads WHERE user_id = $1 AND filename = $2
					RETURNING filename`, userID, avatar).Scan(&consumed); err != nil {
					return err
				}
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET name = $1, username = $2,
			email = $3, avatar = NULLIF($4, ''), bio = NULLIF($5, '') WHERE id = $6`,
			name, username, email, avatar, bio, userID); err != nil {
			return err
		}
		result.Updated = true
		if oldAvatar.Valid && oldAvatar.String != "" && oldAvatar.String != avatar {
			var stillUsed bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (
				  SELECT 1 FROM posts WHERE filename = $1
				  UNION SELECT 1 FROM users WHERE avatar = $1
				) AS exists`, oldAvatar.String).Scan(&stillUsed); err != nil {
				return err
			}
			if !stillUsed {
				result.UnusedAvatar = oldAvatar.String
			}
		}
		userIDInt, err := strconv.ParseInt(userID, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid user id: %w", err)
		}
		bioStr := ""
		if bio != nil {
			bioStr = *bio
		}
		payload, err := database.MarshalOutboxPayload(database.EntityUserUpsertPayload{
			Table:    "users",
			Op:       "upsert",
			ID:       userIDInt,
			UserID:   strconv.FormatInt(userIDInt, 10),
			Name:     name,
			Username: username,
			Bio:      bioStr,
		})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO outbox (topic, payload) VALUES ($1, $2)`, "entity-changes", payload); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO audit_log (user_id, action) VALUES ($1, 'profile_updated')`, userID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return UpdateUserResult{Updated: false}, nil
	}
	if err != nil {
		if database.UniqueViolation(err) {
			return UpdateUserResult{}, store.ErrConflict
		}
		return UpdateUserResult{}, err
	}
	return result, nil
}

func (r *UserRepository) ChangePassword(ctx context.Context, userID, passwordHash, currentSessionID string) error {
	return r.db.Write(ctx, func() error {
		tx, err := r.db.Pool().Begin(ctx)
		if err != nil {
			return err
		}
		defer database.Rollback(ctx, tx)
		if _, err := tx.Exec(ctx, `UPDATE users SET password = $1 WHERE id = $2`, passwordHash, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND id != $2`,
			userID, r.db.HashSession(currentSessionID)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO audit_log (user_id, action) VALUES ($1, 'password_changed')`, userID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	})
}

func (r *UserRepository) ListSuggestedUsers(ctx context.Context, viewerID string, limit int) ([]User, error) {
	var result []User
	err := r.db.Read(ctx, func() error {
		// Match userColumns' NULLIF($1, '')::bigint cast; Postgres infers one
		// type per placeholder, and IS DISTINCT FROM keeps anonymous viewers.
		rows, err := r.db.Pool().Query(ctx, `SELECT `+userColumns+`
            FROM users u
            WHERE u.id IS DISTINCT FROM NULLIF($1, '')::bigint
              AND NOT EXISTS (
                SELECT 1 FROM follows WHERE follower_id = NULLIF($1, '')::bigint AND followee_id = u.id
              )
            ORDER BY u.follower_count DESC
            LIMIT $2`, viewerID, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		result = []User{}
		for rows.Next() {
			var user User
			var avatar, bio sql.NullString
			if err := rows.Scan(&user.ID, &user.Name, &user.Username,
				&user.Email, &avatar, &bio, &user.Posts, &user.Likes,
				&user.Followers, &user.Following, &user.IsFollowing,
				&user.Created, &user.PublicID); err != nil {
				return err
			}
			user.Avatar = database.NullableString(avatar)
			user.Bio = database.NullableString(bio)
			result = append(result, user)
		}
		return rows.Err()
	})
	return result, err
}

var _ Repository = (*UserRepository)(nil)
