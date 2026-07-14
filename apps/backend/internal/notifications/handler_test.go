package notifications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"phasma/backend/internal/httpx"
	"phasma/backend/internal/pagination"
	"phasma/backend/internal/store"
)

const testUserID = "42"
const testPublicID = "550e8400-e29b-41d4-a716-446655440000"

type fakeApplication struct {
	items       []Notification
	nextCursor  *pagination.Cursor
	listErr     error
	unreadCount int
	unreadErr   error
	markReadErr error

	requestedUserID  int64
	requestedCursor  *pagination.Cursor
	requestedLimit   int
	markReadPublicID string
	markReadUserID   int64
}

func (a *fakeApplication) ListNotifications(_ context.Context, query ListQuery) ([]Notification, *pagination.Cursor, error) {
	a.requestedUserID = query.UserID
	a.requestedCursor = query.Cursor
	a.requestedLimit = query.Limit
	return a.items, a.nextCursor, a.listErr
}

func (a *fakeApplication) MarkRead(_ context.Context, publicID string, userID int64) error {
	a.markReadPublicID = publicID
	a.markReadUserID = userID
	return a.markReadErr
}

func (a *fakeApplication) UnreadCount(_ context.Context, userID int64) (int, error) {
	a.requestedUserID = userID
	return a.unreadCount, a.unreadErr
}

func TestListNotificationsRequiresAuth(t *testing.T) {
	handler := Handler{Service: &fakeApplication{}}
	req := httptest.NewRequest(http.MethodGet, "/notifications", nil)
	res := httptest.NewRecorder()

	handler.ListNotifications(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestListNotificationsRejectsInvalidPagination(t *testing.T) {
	handler := Handler{Service: &fakeApplication{}}
	req := httpx.WithUserID(httptest.NewRequest(http.MethodGet, "/notifications?limit=abc", nil), testUserID)
	res := httptest.NewRecorder()

	handler.ListNotifications(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func TestListNotificationsPassesUserIDAndPagination(t *testing.T) {
	app := &fakeApplication{items: []Notification{{PublicID: testPublicID}}}
	handler := Handler{Service: app}
	cursor := pagination.Cursor{Created: time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC), ID: 7}
	req := httpx.WithUserID(
		httptest.NewRequest(http.MethodGet, "/notifications?cursor="+pagination.EncodeCursor(cursor)+"&limit=25", nil),
		testUserID,
	)
	res := httptest.NewRecorder()

	handler.ListNotifications(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if app.requestedUserID != 42 || app.requestedCursor == nil || *app.requestedCursor != cursor || app.requestedLimit != 25 {
		t.Fatalf("request = user %d cursor %+v limit %d", app.requestedUserID, app.requestedCursor, app.requestedLimit)
	}
}

func TestUnreadCountRequiresAuth(t *testing.T) {
	handler := Handler{Service: &fakeApplication{}}
	req := httptest.NewRequest(http.MethodGet, "/notifications/unread-count", nil)
	res := httptest.NewRecorder()

	handler.UnreadCount(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestUnreadCountReturnsCount(t *testing.T) {
	app := &fakeApplication{unreadCount: 3}
	handler := Handler{Service: app}
	req := httpx.WithUserID(httptest.NewRequest(http.MethodGet, "/notifications/unread-count", nil), testUserID)
	res := httptest.NewRecorder()

	handler.UnreadCount(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if body := res.Body.String(); body != `{"count":3}`+"\n" {
		t.Fatalf("body = %q", body)
	}
}

func TestMarkReadRequiresAuth(t *testing.T) {
	handler := Handler{Service: &fakeApplication{}}
	req := httptest.NewRequest(http.MethodPut, "/notifications/"+testPublicID+"/read", nil)
	req.SetPathValue("id", testPublicID)
	res := httptest.NewRecorder()

	handler.MarkRead(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestMarkReadRejectsInvalidUUID(t *testing.T) {
	handler := Handler{Service: &fakeApplication{}}
	req := httpx.WithUserID(httptest.NewRequest(http.MethodPut, "/notifications/not-a-uuid/read", nil), testUserID)
	req.SetPathValue("id", "not-a-uuid")
	res := httptest.NewRecorder()

	handler.MarkRead(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func TestMarkReadMapsNotFound(t *testing.T) {
	app := &fakeApplication{markReadErr: store.ErrNotFound}
	handler := Handler{Service: app}
	req := httpx.WithUserID(httptest.NewRequest(http.MethodPut, "/notifications/"+testPublicID+"/read", nil), testUserID)
	req.SetPathValue("id", testPublicID)
	res := httptest.NewRecorder()

	handler.MarkRead(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestMarkReadSucceeds(t *testing.T) {
	app := &fakeApplication{}
	handler := Handler{Service: app}
	req := httpx.WithUserID(httptest.NewRequest(http.MethodPut, "/notifications/"+testPublicID+"/read", nil), testUserID)
	req.SetPathValue("id", testPublicID)
	res := httptest.NewRecorder()

	handler.MarkRead(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
	if app.markReadPublicID != testPublicID || app.markReadUserID != 42 {
		t.Fatalf("MarkRead called with (%q, %d), want (%q, 42)", app.markReadPublicID, app.markReadUserID, testPublicID)
	}
}
