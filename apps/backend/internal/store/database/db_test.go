package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"phasma/backend/internal/resilience"
	"phasma/backend/internal/store"
)

func TestUniqueViolation(t *testing.T) {
	if !UniqueViolation(&pgconn.PgError{Code: "23505"}) {
		t.Fatal("UniqueViolation() = false for code 23505")
	}
	if UniqueViolation(&pgconn.PgError{Code: "23503"}) {
		t.Fatal("UniqueViolation() = true for a foreign key violation")
	}
	if UniqueViolation(errors.New("boom")) {
		t.Fatal("UniqueViolation() = true for a non-pg error")
	}
}

func TestNullableString(t *testing.T) {
	if got := NullableString(sql.NullString{}); got != nil {
		t.Fatalf("NullableString(invalid) = %v, want nil", got)
	}
	got := NullableString(sql.NullString{String: "value", Valid: true})
	if got == nil || *got != "value" {
		t.Fatalf("NullableString(valid) = %v, want pointer to \"value\"", got)
	}
}

func newTestDB(maxAttempts, failureThreshold int) *DB {
	return &DB{
		breaker: resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
			Name:             "test",
			FailureThreshold: failureThreshold,
			Cooldown:         time.Minute,
			IsFailure:        isTransientDatabaseError,
		}),
		retryCfg: resilience.RetryConfig{
			MaxAttempts: maxAttempts,
			BaseBackoff: time.Millisecond,
			IsRetryable: isTransientDatabaseError,
		},
	}
}

func TestDBReadRetriesTransientErrorsUntilSuccess(t *testing.T) {
	db := newTestDB(3, 5)

	attempts := 0
	err := db.Read(context.Background(), func() error {
		attempts++
		if attempts < 2 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestDBReadTreatsNoRowsAsHealthyNotAFailure(t *testing.T) {
	db := newTestDB(1, 1)

	err := db.Read(context.Background(), func() error { return pgx.ErrNoRows })
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("Read() error = %v, want pgx.ErrNoRows", err)
	}
	if !db.breaker.Allow() {
		t.Fatal("breaker opened after a pgx.ErrNoRows result")
	}
}

func TestDBReadOpensBreakerAndShortCircuitsSubsequentCalls(t *testing.T) {
	db := newTestDB(1, 1)

	if err := db.Read(context.Background(), func() error { return errors.New("down") }); err == nil {
		t.Fatal("Read() error = nil, want the underlying failure")
	}

	called := false
	err := db.Read(context.Background(), func() error {
		called = true
		return nil
	})
	if !errors.Is(err, store.ErrUnavailable) {
		t.Fatalf("Read() error = %v, want store.ErrUnavailable", err)
	}
	if called {
		t.Fatal("fn was invoked while the breaker was open")
	}
}

func TestDBWriteDoesNotRetryEvenOnTransientErrors(t *testing.T) {
	db := newTestDB(3, 5)

	calls := 0
	wantErr := errors.New("transient")
	err := db.Write(context.Background(), func() error {
		calls++
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Write() error = %v, want %v", err, wantErr)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (writes must not retry)", calls)
	}
}
