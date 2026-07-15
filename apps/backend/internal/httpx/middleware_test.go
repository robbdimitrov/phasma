package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testValidSessionID = "AAAAAAAAAAAAAAAAAAAAAAAAAAAA"

type fakeOptionalSessionStore struct {
	session Session
	err     error
}

func (s *fakeOptionalSessionStore) RefreshSession(context.Context, string) (Session, error) {
	return s.session, s.err
}

func TestOptionalSessionPopulatesUserIDForValidSession(t *testing.T) {
	store := &fakeOptionalSessionStore{session: Session{ID: "hashed", UserID: "42"}}

	var gotUserID string
	var gotOK bool
	handler := OptionalSession(store)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotUserID, gotOK = UserID(r)
	}))

	req := httptest.NewRequest(http.MethodGet, "/posts/popular", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: testValidSessionID})
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !gotOK || gotUserID != "42" {
		t.Fatalf("UserID() = (%q, %v), want (\"42\", true)", gotUserID, gotOK)
	}
}

func TestOptionalSessionProceedsAnonymouslyWithoutCookie(t *testing.T) {
	store := &fakeOptionalSessionStore{session: Session{ID: "hashed", UserID: "42"}}

	called := false
	var gotOK bool
	handler := OptionalSession(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, gotOK = UserID(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/posts/popular", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if !called || res.Code != http.StatusOK {
		t.Fatalf("request without a cookie must still reach the handler, status = %d", res.Code)
	}
	if gotOK {
		t.Fatal("UserID() ok = true, want false for a request with no session cookie")
	}
}

func TestOptionalSessionProceedsAnonymouslyForMalformedCookie(t *testing.T) {
	store := &fakeOptionalSessionStore{session: Session{ID: "hashed", UserID: "42"}}

	var gotOK bool
	handler := OptionalSession(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, gotOK = UserID(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/posts/popular", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "not-a-valid-session"})
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("malformed session cookie must not block the request, status = %d", res.Code)
	}
	if gotOK {
		t.Fatal("UserID() ok = true, want false for a malformed session cookie")
	}
}

func TestOptionalSessionProceedsAnonymouslyOnStoreError(t *testing.T) {
	store := &fakeOptionalSessionStore{err: errors.New("database unavailable")}

	var gotOK bool
	handler := OptionalSession(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, gotOK = UserID(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/posts/popular", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: testValidSessionID})
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("a session store error must not block the request, status = %d", res.Code)
	}
	if gotOK {
		t.Fatal("UserID() ok = true, want false when the session store errors")
	}
}

func TestRecoverReturnsInternalServerErrorOnPanic(t *testing.T) {
	handler := Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/posts/popular", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusInternalServerError)
	}
	if ct := res.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want application/json; charset=utf-8", ct)
	}
}

func TestRecoverPassesThroughWithoutPanic(t *testing.T) {
	handler := Recover(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
}
