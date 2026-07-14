package sessions

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"phasma/backend/internal/env"
	"phasma/backend/internal/httpx"
	"phasma/backend/internal/store/database"
)

const (
	sessionTTL                     = 7 * 24 * time.Hour
	defaultSessionAbsoluteTTLHours = 720
	maxSessionsPerUser             = 100
)

type SessionRepository struct {
	db *database.DB
}

func NewSessionRepository(client *database.Client) *SessionRepository {
	return &SessionRepository{db: client.DB()}
}

func (r *SessionRepository) FindLoginCredentialsByEmail(ctx context.Context, email string) (*UserCredentials, error) {
	var credentials UserCredentials
	err := r.db.Read(ctx, func() error {
		return r.db.Pool().QueryRow(ctx, `SELECT id, username, password FROM users WHERE email = $1`, email).
			Scan(&credentials.ID, &credentials.Username, &credentials.PasswordHash)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &credentials, nil
}

func (r *SessionRepository) CreateSession(ctx context.Context, sessionID string, userID int, expiresAt time.Time) (CreatedSession, error) {
	var session CreatedSession
	hashedSessionID := r.db.HashSession(sessionID)
	err := r.db.Write(ctx, func() error {
		tx, err := r.db.Pool().Begin(ctx)
		if err != nil {
			return err
		}
		defer database.Rollback(ctx, tx)

		// Serialize session creation per user so concurrent replicas cannot
		// exceed the bounded active-session policy.
		if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, userID).
			Scan(&session.UserID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM sessions
			WHERE user_id = $1
			  AND (expires_at <= clock_timestamp()
			    OR created < clock_timestamp() - ($2 * interval '1 hour'))`,
			userID, sessionAbsoluteTTLHours()); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO sessions (id, user_id, created, expires_at)
			VALUES ($1, $2, clock_timestamp(), $3) RETURNING user_id`,
			hashedSessionID, userID, expiresAt).Scan(&session.UserID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM sessions
			WHERE user_id = $1
			  AND id <> $2
			  AND id NOT IN (
			  SELECT id FROM sessions
			  WHERE user_id = $1 AND id <> $2
			  ORDER BY created DESC, public_id DESC
			  LIMIT $3
			)`, userID, hashedSessionID, maxSessionsPerUser-1); err != nil {
			return err
		}
		return tx.Commit(ctx)
	})
	if err != nil {
		return CreatedSession{}, err
	}
	return session, nil
}

func (r *SessionRepository) RefreshSession(ctx context.Context, sessionID string) (httpx.Session, error) {
	hashed := r.db.HashSession(sessionID)
	now := time.Now()
	absoluteTTLHours := sessionAbsoluteTTLHours()
	var session httpx.Session
	var userID int
	// Read (retried): the UPDATE only re-stamps expires_at to a deterministic
	// window, so a retry on transient contention is idempotent.
	err := r.db.Read(ctx, func() error {
		return r.db.Pool().QueryRow(ctx, `WITH refreshed AS (
			  UPDATE sessions SET expires_at = $2
			  WHERE id = $1 AND expires_at > now() AND expires_at < $3
			    AND created >= now() - ($4 * interval '1 hour')
			  RETURNING id, user_id
			)
			SELECT id, user_id FROM refreshed
			UNION ALL
			SELECT id, user_id FROM sessions
			WHERE id = $1 AND expires_at > now()
			  AND created >= now() - ($4 * interval '1 hour')
			  AND NOT EXISTS (SELECT 1 FROM refreshed)
			LIMIT 1`,
			hashed, now.Add(sessionTTL), now.Add(sessionTTL/2), absoluteTTLHours).
			Scan(&session.ID, &userID)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.Session{}, nil
	}
	if err != nil {
		return httpx.Session{}, err
	}
	session.UserID = strconv.Itoa(userID)
	return session, nil
}

func (r *SessionRepository) DeleteExpiredSessions(ctx context.Context) error {
	return r.exec(ctx,
		`DELETE FROM sessions
		WHERE expires_at <= now() OR created < now() - ($1 * interval '1 hour')`,
		sessionAbsoluteTTLHours(),
	)
}

func sessionAbsoluteTTLHours() int {
	return env.Int("SESSION_ABSOLUTE_TTL_HOURS", defaultSessionAbsoluteTTLHours)
}

func (r *SessionRepository) DeleteSession(ctx context.Context, sessionID string) error {
	return r.exec(ctx, `DELETE FROM sessions WHERE id = $1`, r.db.HashSession(sessionID))
}

func (r *SessionRepository) ListActiveSessions(ctx context.Context, userID, currentSessionToken string) ([]Session, error) {
	var items []Session
	err := r.db.Read(ctx, func() error {
		attemptItems := make([]Session, 0)
		rows, err := r.db.Pool().Query(ctx, `SELECT public_id, created,
			  LEAST(expires_at, created + ($3 * interval '1 hour')), id = $2
			FROM sessions
			WHERE user_id = $1
			  AND expires_at > now()
			  AND created >= now() - ($3 * interval '1 hour')
			ORDER BY id = $2 DESC, created DESC, public_id DESC
			LIMIT $4`,
			userID, r.db.HashSession(currentSessionToken), sessionAbsoluteTTLHours(), maxSessionsPerUser)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var session Session
			if err := rows.Scan(&session.ID, &session.Created, &session.ExpiresAt, &session.Current); err != nil {
				return err
			}
			attemptItems = append(attemptItems, session)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		items = attemptItems
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SessionRepository) DeleteSessionByID(
	ctx context.Context,
	publicID, userID, currentSessionToken string,
) (DeleteSessionOutcome, error) {
	var result string
	err := r.db.Write(ctx, func() error {
		return r.db.Pool().QueryRow(ctx, `WITH deleted AS (
			  DELETE FROM sessions
			  WHERE public_id = $1 AND user_id = $2 AND id <> $3
			  RETURNING 1
			)
			SELECT CASE
			  WHEN EXISTS (SELECT 1 FROM deleted) THEN 'deleted'
			  WHEN EXISTS (
			    SELECT 1 FROM sessions
			    WHERE public_id = $1 AND user_id = $2 AND id = $3
			  ) THEN 'current'
			  ELSE 'not_found'
			END`,
			publicID, userID, r.db.HashSession(currentSessionToken)).Scan(&result)
	})
	if err != nil {
		return DeleteSessionNotFound, err
	}
	switch result {
	case "deleted":
		return DeleteSessionDeleted, nil
	case "current":
		return DeleteSessionCurrent, nil
	default:
		return DeleteSessionNotFound, nil
	}
}

func (r *SessionRepository) UpdatePasswordHash(ctx context.Context, userID int, hash string) error {
	return r.exec(ctx, `UPDATE users SET password = $1 WHERE id = $2`, hash, userID)
}

func (r *SessionRepository) exec(ctx context.Context, query string, args ...any) error {
	return r.db.Write(ctx, func() error {
		_, err := r.db.Pool().Exec(ctx, query, args...)
		return err
	})
}

var (
	_ Repository         = (*SessionRepository)(nil)
	_ httpx.SessionStore = (*SessionRepository)(nil)
)
