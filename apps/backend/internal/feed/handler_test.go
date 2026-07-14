package feed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"phasma/backend/internal/httpx"
	"phasma/backend/internal/pagination"
	"phasma/backend/internal/posts"
)

const testUserID = "42"

type fakeApplication struct {
	items      []posts.Post
	nextCursor *pagination.Cursor
	err        error

	requestedUserID string
	requestedCursor *pagination.Cursor
	requestedLimit  int
}

func (a *fakeApplication) ListFeed(_ context.Context, userID string, cursor *pagination.Cursor, limit int) ([]posts.Post, *pagination.Cursor, error) {
	a.requestedUserID = userID
	a.requestedCursor = cursor
	a.requestedLimit = limit
	return a.items, a.nextCursor, a.err
}

func TestGetFeedRequiresAuth(t *testing.T) {
	handler := Handler{Service: &fakeApplication{}}
	req := httptest.NewRequest(http.MethodGet, "/feed", nil)
	res := httptest.NewRecorder()

	handler.GetFeed(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestGetFeedRejectsInvalidPagination(t *testing.T) {
	handler := Handler{Service: &fakeApplication{}}
	req := httpx.WithUserID(httptest.NewRequest(http.MethodGet, "/feed?limit=abc", nil), testUserID)
	res := httptest.NewRecorder()

	handler.GetFeed(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func TestGetFeedPassesUserIDAndPagination(t *testing.T) {
	app := &fakeApplication{items: []posts.Post{{ID: 1}}}
	handler := Handler{Service: app}
	cursor := pagination.Cursor{Created: time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC), ID: 9}
	req := httpx.WithUserID(
		httptest.NewRequest(http.MethodGet, "/feed?cursor="+pagination.EncodeCursor(cursor)+"&limit=30", nil),
		testUserID,
	)
	res := httptest.NewRecorder()

	handler.GetFeed(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if app.requestedUserID != testUserID || app.requestedCursor == nil || *app.requestedCursor != cursor || app.requestedLimit != 30 {
		t.Fatalf("request = user %q cursor %+v limit %d", app.requestedUserID, app.requestedCursor, app.requestedLimit)
	}
}

func TestGetFeedReturnsEmptyItemsWhenFeedEmpty(t *testing.T) {
	app := &fakeApplication{items: []posts.Post{}}
	handler := Handler{Service: app}
	req := httpx.WithUserID(httptest.NewRequest(http.MethodGet, "/feed", nil), testUserID)
	res := httptest.NewRecorder()

	handler.GetFeed(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if body := res.Body.String(); body != `{"items":[],"nextCursor":null}`+"\n" {
		t.Fatalf("body = %q", body)
	}
}
