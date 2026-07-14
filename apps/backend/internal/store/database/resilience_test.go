package database

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"phasma/backend/internal/store"
)

func TestIsTransientDatabaseError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"no rows", pgx.ErrNoRows, false},
		{"not found", store.ErrNotFound, false},
		{"forbidden", store.ErrForbidden, false},
		{"conflict", store.ErrConflict, false},
		{"serialization failure", &pgconn.PgError{Code: "40001"}, true},
		{"deadlock detected", &pgconn.PgError{Code: "40P01"}, true},
		{"admin shutdown", &pgconn.PgError{Code: "57P01"}, true},
		{"crash shutdown", &pgconn.PgError{Code: "57P02"}, true},
		{"unique violation is not retried", &pgconn.PgError{Code: "23505"}, false},
		{"unclassified error defaults to retryable", errors.New("boom"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransientDatabaseError(tt.err); got != tt.want {
				t.Fatalf("isTransientDatabaseError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
