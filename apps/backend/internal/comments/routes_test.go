package comments

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"phasma/backend/internal/httpx"
)

func TestRegisterRoutes(t *testing.T) {
	handler := Handler{Service: NewService(&fakeStore{})}
	public := http.NewServeMux()
	protected := http.NewServeMux()
	RegisterPublicRoutes(public, handler)
	RegisterProtectedRoutes(protected, handler)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/posts/"+testPublicID+"/comments", nil)
	public.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("public route status = %d, want %d", res.Code, http.StatusNotFound)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/posts/"+testPublicID+"/comments", strings.NewReader(`{"body":"hi"}`))
	public.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("protected route reachable on public mux without session, status = %d", res.Code)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/posts/"+testPublicID+"/comments", nil)
	protected.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("protected read route status = %d, want %d", res.Code, http.StatusOK)
	}

	res = httptest.NewRecorder()
	req = httpx.WithUserID(httptest.NewRequest(http.MethodPost, "/posts/"+testPublicID+"/comments", strings.NewReader(`{"body":"hi"}`)), "1")
	protected.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("protected route status = %d, want %d", res.Code, http.StatusCreated)
	}
}
