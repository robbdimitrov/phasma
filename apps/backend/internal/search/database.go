package search

import (
	"context"
	"database/sql"

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
			LIMIT 8`, q)
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
			LIMIT 8`, q)
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

var _ Repository = (*SearchRepository)(nil)
