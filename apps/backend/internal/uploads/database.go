package uploads

import (
	"context"

	"phasma/backend/internal/store/database"
)

const maxPendingUploads = 20

type UploadRepository struct {
	db *database.DB
}

func NewUploadRepository(client *database.Client) *UploadRepository {
	return &UploadRepository{db: client.DB()}
}

func (r *UploadRepository) CreateUpload(ctx context.Context, userID, filename string) (bool, []string, error) {
	var created bool
	var expired []string
	err := r.db.Write(ctx, func() error {
		tx, err := r.db.Pool().Begin(ctx)
		if err != nil {
			return err
		}
		defer database.Rollback(ctx, tx)

		rows, err := tx.Query(ctx, `DELETE FROM uploads
			WHERE user_id = $1 AND created <= now() - $2::interval RETURNING filename`,
			userID, "1 hour")
		if err != nil {
			return err
		}
		expired = []string{}
		for rows.Next() {
			var f string
			if err := rows.Scan(&f); err != nil {
				rows.Close()
				return err
			}
			expired = append(expired, f)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		// Lock remaining rows and count atomically before inserting.
		var count int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM (SELECT filename FROM uploads WHERE user_id = $1 FOR UPDATE) locked`,
			userID).Scan(&count); err != nil {
			return err
		}
		if count >= maxPendingUploads {
			return tx.Commit(ctx)
		}

		tag, err := tx.Exec(ctx,
			`INSERT INTO uploads (user_id, filename) VALUES ($1, $2)`, userID, filename)
		if err != nil {
			return err
		}
		created = tag.RowsAffected() > 0
		return tx.Commit(ctx)
	})
	return created, expired, err
}

var _ Repository = (*UploadRepository)(nil)
