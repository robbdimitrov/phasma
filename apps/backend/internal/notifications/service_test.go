package notifications

import (
	"context"
	"testing"
	"time"

	"phasma/backend/internal/pagination"
)

type fakeRepository struct {
	items []Notification
}

func (r *fakeRepository) ListNotifications(context.Context, int64, *pagination.Cursor, int) ([]Notification, error) {
	return r.items, nil
}
func (r *fakeRepository) MarkRead(context.Context, string, int64) error        { return nil }
func (r *fakeRepository) UnreadCount(context.Context, int64) (int, error)      { return 0, nil }
func (r *fakeRepository) DeleteByEntity(context.Context, string, string) error { return nil }
func (r *fakeRepository) DeleteByActorAndType(context.Context, int64, int64, string, string) error {
	return nil
}
func (r *fakeRepository) CreateNotification(context.Context, Notification) error { return nil }

func TestServiceListNotificationsReturnsNextCursorWhenMoreExist(t *testing.T) {
	created := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	// The service requests limit+1 from the repository to detect "has more";
	// three items for a limit of 2 simulates that extra row being present.
	repo := &fakeRepository{items: []Notification{
		{ID: 3, Created: created},
		{ID: 2, Created: created},
		{ID: 1, Created: created},
	}}
	service := NewService(repo)

	got, cursor, err := service.ListNotifications(context.Background(), ListQuery{UserID: 1, Limit: 2})
	if err != nil {
		t.Fatalf("ListNotifications() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(got))
	}
	if cursor == nil || cursor.ID != 2 {
		t.Fatalf("cursor = %+v, want ID 2", cursor)
	}
}

func TestServiceListNotificationsOmitsCursorWhenExhausted(t *testing.T) {
	repo := &fakeRepository{items: []Notification{{ID: 1}}}
	service := NewService(repo)

	got, cursor, err := service.ListNotifications(context.Background(), ListQuery{UserID: 1, Limit: 10})
	if err != nil {
		t.Fatalf("ListNotifications() error = %v", err)
	}
	if len(got) != 1 || cursor != nil {
		t.Fatalf("items = %d cursor = %+v, want 1 item and nil cursor", len(got), cursor)
	}
}

func TestServiceListNotificationsOmitsCursorForEmptyResult(t *testing.T) {
	repo := &fakeRepository{items: []Notification{}}
	service := NewService(repo)

	got, cursor, err := service.ListNotifications(context.Background(), ListQuery{UserID: 1, Limit: 10})
	if err != nil {
		t.Fatalf("ListNotifications() error = %v", err)
	}
	if len(got) != 0 || cursor != nil {
		t.Fatalf("items = %d cursor = %+v, want 0 items and nil cursor", len(got), cursor)
	}
}
