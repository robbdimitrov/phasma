package search

import (
	"context"
	"database/sql"

	"phasma/backend/internal/store"
	"phasma/backend/internal/store/database"
)

type SearchRepository struct {
	db *database.DB
}

func NewSearchRepository(client *database.Client) *SearchRepository {
	return &SearchRepository{db: client.DB()}
}

func (r *SearchRepository) SearchUsers(ctx context.Context, q string) ([]UserResult, error) {
	var results []UserResult
	err := r.db.Read(ctx, func() error {
		rows, err := r.db.Pool().Query(ctx,
			`SELECT username, name, avatar FROM users
			WHERE username % $1
			ORDER BY similarity(username, $1) DESC, username
			LIMIT $2`, q, typeaheadLen)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []UserResult{}
		for rows.Next() {
			var u UserResult
			var avatar sql.NullString
			if err := rows.Scan(&u.Username, &u.Name, &avatar); err != nil {
				return err
			}
			u.Avatar = database.NullableString(avatar)
			results = append(results, u)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (r *SearchRepository) SearchHashtags(ctx context.Context, q string) ([]HashtagResult, error) {
	var results []HashtagResult
	err := r.db.Read(ctx, func() error {
		rows, err := r.db.Pool().Query(ctx,
			`SELECT h.name, h.post_count
			FROM hashtags h
			WHERE h.name % $1
			ORDER BY similarity(h.name, $1) DESC, h.name
			LIMIT $2`, q, typeaheadLen)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []HashtagResult{}
		for rows.Next() {
			var h HashtagResult
			if err := rows.Scan(&h.Name, &h.PostCount); err != nil {
				return err
			}
			results = append(results, h)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// FollowingUsernames reports which of usernames the viewer follows. Mirrors
// the NULLIF($n, empty string)::bigint convention used elsewhere for
// comparing a session-derived viewer id against a bigint column.
func (r *SearchRepository) FollowingUsernames(ctx context.Context, viewerID string, usernames []string) (map[string]bool, error) {
	following := map[string]bool{}
	if len(usernames) == 0 {
		return following, nil
	}
	err := r.db.Read(ctx, func() error {
		rows, err := r.db.Pool().Query(ctx,
			`SELECT u.username FROM follows f
			JOIN users u ON u.id = f.followee_id
			WHERE f.follower_id = NULLIF($1, '')::bigint AND u.username = ANY($2)`,
			viewerID, usernames)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var username string
			if err := rows.Scan(&username); err != nil {
				return err
			}
			following[username] = true
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return following, nil
}

// PostLikeCounts hydrates like_count at read time, mirroring
// ListRecentSearches's pattern below.
func (r *SearchRepository) PostLikeCounts(ctx context.Context, postIDs []string) (map[string]int, error) {
	counts := map[string]int{}
	if len(postIDs) == 0 {
		return counts, nil
	}
	err := r.db.Read(ctx, func() error {
		rows, err := r.db.Pool().Query(ctx,
			`SELECT public_id::text, like_count FROM posts WHERE public_id = ANY($1::uuid[])`, postIDs)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var likes int
			if err := rows.Scan(&id, &likes); err != nil {
				return err
			}
			counts[id] = likes
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return counts, nil
}

// recentSearchLimit caps how many recent-search rows are retained per user.
// A separate constant from typeaheadLen even though both are currently 10:
// this bounds a persisted recall list, not a live relevance list, so the two
// are free to diverge again without a design change.
const recentSearchLimit = 10

// RecordRecentSearch upserts the entry (bumping created on a repeat search
// instead of duplicating it), then trims the user's history back to
// recentSearchLimit in the same transaction.
func (r *SearchRepository) RecordRecentSearch(ctx context.Context, userID, entityType, reference string) error {
	return r.db.Write(ctx, func() error {
		tx, err := r.db.Pool().Begin(ctx)
		if err != nil {
			return err
		}
		defer database.Rollback(ctx, tx)

		// Serialize per user so two concurrent requests (e.g. two tabs) can't
		// each compute the trim's "top N" from a stale snapshot and leave more
		// than recentSearchLimit rows behind. Mirrors sessions.CreateSession's
		// per-user row lock for the same class of bounded-list problem.
		var lockedUserID string
		if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE id = $1::bigint FOR UPDATE`, userID).
			Scan(&lockedUserID); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO recent_searches (user_id, entity_type, reference)
			VALUES ($1::bigint, $2, $3)
			ON CONFLICT (user_id, entity_type, reference) DO UPDATE SET created = now()`,
			userID, entityType, reference); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx,
			`DELETE FROM recent_searches
			WHERE user_id = $1::bigint AND id NOT IN (
				SELECT id FROM recent_searches
				WHERE user_id = $1::bigint
				ORDER BY created DESC, id DESC
				LIMIT $2
			)`, userID, recentSearchLimit); err != nil {
			return err
		}

		return tx.Commit(ctx)
	})
}

// ListRecentSearches hydrates each row's current username/name/avatar or
// hashtag name/post_count at read time rather than trusting a denormalized
// snapshot; a reference that no longer resolves (e.g. a deleted account) is
// silently excluded instead of surfacing stale data.
func (r *SearchRepository) ListRecentSearches(ctx context.Context, userID string) ([]RecentSearchItem, error) {
	var results []RecentSearchItem
	err := r.db.Read(ctx, func() error {
		rows, err := r.db.Pool().Query(ctx,
			`SELECT r.public_id::text, r.entity_type, r.reference,
				u.username, u.name, u.avatar, h.name, h.post_count
			FROM recent_searches r
			LEFT JOIN users u ON u.username = r.reference AND r.entity_type = 'users'
			LEFT JOIN hashtags h ON h.name = r.reference AND r.entity_type = 'hashtags'
			WHERE r.user_id = $1::bigint
				AND (r.entity_type = 'posts' OR u.id IS NOT NULL OR h.id IS NOT NULL)
			ORDER BY r.created DESC, r.id DESC
			LIMIT $2`, userID, recentSearchLimit)
		if err != nil {
			return err
		}
		defer rows.Close()
		results = []RecentSearchItem{}
		for rows.Next() {
			var id, entityType, reference string
			var username, name, avatar sql.NullString
			var hashtagName sql.NullString
			var postCount sql.NullInt32
			if err := rows.Scan(&id, &entityType, &reference,
				&username, &name, &avatar, &hashtagName, &postCount); err != nil {
				return err
			}
			item := RecentSearchItem{ID: id, Type: entityType}
			switch entityType {
			case "users":
				item.Item = UserResult{
					Username: username.String,
					Name:     name.String,
					Avatar:   database.NullableString(avatar),
				}
			case "hashtags":
				item.Item = HashtagResult{Name: hashtagName.String, PostCount: int(postCount.Int32)}
			default:
				item.Item = reference
			}
			results = append(results, item)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (r *SearchRepository) DeleteRecentSearch(ctx context.Context, userID, publicID string) error {
	return r.db.Write(ctx, func() error {
		tag, err := r.db.Pool().Exec(ctx,
			`DELETE FROM recent_searches WHERE public_id = $1 AND user_id = $2::bigint`,
			publicID, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return store.ErrNotFound
		}
		return nil
	})
}

func (r *SearchRepository) ClearRecentSearches(ctx context.Context, userID string) error {
	return r.db.Write(ctx, func() error {
		_, err := r.db.Pool().Exec(ctx, `DELETE FROM recent_searches WHERE user_id = $1::bigint`, userID)
		return err
	})
}

var _ Repository = (*SearchRepository)(nil)
