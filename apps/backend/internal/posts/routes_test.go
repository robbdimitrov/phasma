package posts

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"phasma/backend/internal/httpx"
)

func TestRegisterRoutes(t *testing.T) {
	store := &fakeStore{
		post:  Post{ID: 1, PublicID: testPublicID, Filename: "a"},
		found: true, createdID: testPublicID, created: true,
	}
	handler := Handler{Service: NewService(store, nil)}
	public := http.NewServeMux()
	protected := http.NewServeMux()
	RegisterPublicRoutes(public, handler)
	RegisterProtectedRoutes(protected, handler)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/posts/"+testPublicID, nil)
	public.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("public route status = %d, want %d", res.Code, http.StatusOK)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(`{"filename":"upload"}`))
	public.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("protected route reachable on public mux without session, status = %d", res.Code)
	}

	res = httptest.NewRecorder()
	req = httpx.WithUserID(httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(`{"filename":"upload"}`)), "1")
	protected.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("protected route status = %d, want %d", res.Code, http.StatusCreated)
	}
}
