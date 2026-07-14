package database

import (
	"testing"
	"time"

	"phasma/backend/internal/pagination"
)

func TestCursorValuesReturnsZeroValuesForNilCursor(t *testing.T) {
	hasCursor, created, id := CursorValues(nil)
	if hasCursor {
		t.Fatal("hasCursor = true for nil cursor")
	}
	if !created.IsZero() {
		t.Fatalf("created = %v, want zero time", created)
	}
	if id != 0 {
		t.Fatalf("id = %d, want 0", id)
	}
}

func TestCursorValuesReturnsCursorFields(t *testing.T) {
	created := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	cursor := &pagination.Cursor{Created: created, ID: 42}

	hasCursor, gotCreated, gotID := CursorValues(cursor)
	if !hasCursor {
		t.Fatal("hasCursor = false for non-nil cursor")
	}
	if !gotCreated.Equal(created) {
		t.Fatalf("created = %v, want %v", gotCreated, created)
	}
	if gotID != 42 {
		t.Fatalf("id = %d, want 42", gotID)
	}
}
