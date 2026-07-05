package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"phasma/backend/internal/auth"
	"phasma/backend/internal/env"
	"phasma/backend/internal/resilience"
	"phasma/backend/internal/store"
)

type DB struct {
	pool          *pgxpool.Pool
	sessionSecret string
	breaker       *resilience.CircuitBreaker
	retryCfg      resilience.RetryConfig
}

func Open(ctx context.Context, databaseURL, sessionSecret string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	config.MaxConns = int32(env.Int("POSTGRES_MAX_CONNS", 10))
	config.MaxConnIdleTime = 5 * time.Minute
	config.MaxConnLifetime = time.Duration(env.Int("POSTGRES_CONN_LIFETIME_MINUTES", 30)) * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return &DB{
		pool:          pool,
		sessionSecret: sessionSecret,
		breaker:       newCircuitBreaker(),
		retryCfg:      defaultRetryConfig(),
	}, nil
}

func (db *DB) Close() {
	db.pool.Close()
}

func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// Read runs a read-only operation through the circuit breaker with retry.
// pgx.ErrNoRows is treated as a healthy outcome, not a breaker failure, so
// callers can inspect the returned error for it without tripping the breaker.
func (db *DB) Read(ctx context.Context, fn func() error) error {
	if !db.breaker.Allow() {
		return store.ErrUnavailable
	}
	err := resilience.WithRetry(ctx, db.retryCfg, "database", fn)
	db.record(err)
	return err
}

// Write runs a mutating operation through the circuit breaker without retry,
// since writes may be non-idempotent.
func (db *DB) Write(ctx context.Context, fn func() error) error {
	if !db.breaker.Allow() {
		return store.ErrUnavailable
	}
	err := fn()
	db.record(err)
	return err
}

func (db *DB) record(err error) {
	if err == nil || errors.Is(err, pgx.ErrNoRows) {
		db.breaker.Success()
		return
	}
	db.breaker.Failure(err)
}

func (db *DB) HashSession(sessionID string) string {
	return auth.HashSessionToken(sessionID, db.sessionSecret)
}

func UniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func NullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func Rollback(ctx context.Context, tx pgx.Tx) {
	_ = tx.Rollback(ctx)
}
