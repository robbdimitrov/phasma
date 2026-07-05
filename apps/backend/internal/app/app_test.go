package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"phasma/backend/internal/httpx"
	"phasma/backend/internal/pipeline"
)

func TestRouteContract(t *testing.T) {
	want := []Route{
		{Method: "GET", Path: "/health"},
		{Method: "GET", Path: "/health/background"},
		{Method: "GET", Path: "/ready"},
		{Method: "POST", Path: "/users"},
		{Method: "GET", Path: "/users/{username}/followers"},
		{Method: "GET", Path: "/users/{username}/following"},
		{Method: "GET", Path: "/users/{username}"},
		{Method: "GET", Path: "/users/me", Authenticated: true},
		{Method: "GET", Path: "/users/suggested", Authenticated: true},
		{Method: "GET", Path: "/users/search", Authenticated: true},
		{Method: "POST", Path: "/sessions"},
		{Method: "DELETE", Path: "/sessions"},
		{Method: "GET", Path: "/uploads/"},
		{Method: "GET", Path: "/posts/popular"},
		{Method: "GET", Path: "/users/{username}/posts"},
		{Method: "GET", Path: "/users/{username}/likes"},
		{Method: "GET", Path: "/posts/{publicId}"},
		{Method: "GET", Path: "/posts/{publicId}/comments"},
		{Method: "PUT", Path: "/users/me", Authenticated: true},
		{Method: "POST", Path: "/users/{username}/follow", Authenticated: true},
		{Method: "DELETE", Path: "/users/{username}/follow", Authenticated: true},
		{Method: "GET", Path: "/sessions", Authenticated: true},
		{Method: "DELETE", Path: "/sessions/{sessionId}", Authenticated: true},
		{Method: "POST", Path: "/uploads", Authenticated: true},
		{Method: "POST", Path: "/posts", Authenticated: true},
		{Method: "DELETE", Path: "/posts/{publicId}", Authenticated: true},
		{Method: "POST", Path: "/posts/{publicId}/likes", Authenticated: true},
		{Method: "DELETE", Path: "/posts/{publicId}/likes", Authenticated: true},
		{Method: "POST", Path: "/posts/{publicId}/comments", Authenticated: true},
		{Method: "DELETE", Path: "/posts/{publicId}/comments/{commentId}", Authenticated: true},
		{Method: "GET", Path: "/hashtags/search", Authenticated: true},
		{Method: "GET", Path: "/search", Authenticated: true},
		{Method: "GET", Path: "/feed", Authenticated: true},
		{Method: "GET", Path: "/notifications", Authenticated: true},
		{Method: "GET", Path: "/notifications/unread-count", Authenticated: true},
		{Method: "PUT", Path: "/notifications/{id}/read", Authenticated: true},
	}

	got := Routes()
	if len(got) != len(want) {
		t.Fatalf("route count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("route[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

type fakeSessionStore struct {
	refreshSession httpx.Session
	refreshErr     error
	refreshCalls   int
}

func (store *fakeSessionStore) RefreshSession(_ context.Context, _ string) (httpx.Session, error) {
	store.refreshCalls++
	return store.refreshSession, store.refreshErr
}

func TestHealthEndpointNeverTouchesSessionStore(t *testing.T) {
	store := &fakeSessionStore{refreshSession: httpx.Session{ID: "hashed-session-id", UserID: "1"}}
	app := New(Config{}, Repositories{SessionAuth: store})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "AAAAAAAAAAAAAAAAAAAAAAAAAAAA"})
	app.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
	if store.refreshCalls != 0 {
		t.Fatalf("refresh calls = %d, want 0 -- health checks must stay exempt from authentication even with a cookie present", store.refreshCalls)
	}
}

func TestPersonalizedPublicRouteResolvesOptionalSessionButHealthDoesNot(t *testing.T) {
	var optionalCalls int
	spyOptional := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			optionalCalls++
			w.WriteHeader(http.StatusNoContent)
		})
	}

	public := http.NewServeMux()
	protected := http.NewServeMux()
	registerRoutes(
		routeMux{mux: public},
		routeMux{mux: protected, authenticated: true},
		handlers{},
		nil,
		spyOptional,
	)

	res := httptest.NewRecorder()
	public.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/health", nil))
	if optionalCalls != 0 {
		t.Fatalf("GET /health optionalCalls = %d, want 0 -- health checks must stay exempt from session lookups", optionalCalls)
	}

	res = httptest.NewRecorder()
	public.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/posts/popular", nil))
	if optionalCalls != 1 {
		t.Fatalf("GET /posts/popular optionalCalls = %d, want 1 -- a personalized public route must resolve the viewer id", optionalCalls)
	}
}

// TestLiteralRoutesAreNotShadowedByUsernameWildcard guards against the public
// "GET /users/{username}" wildcard silently swallowing these literal,
// session-required paths -- as it did for all three until this test was
// added (GET /users/search was found shadowed live, after /users/me and
// /users/suggested had already been fixed the same way).
func TestLiteralRoutesAreNotShadowedByUsernameWildcard(t *testing.T) {
	for _, path := range []string{"/users/me", "/users/suggested", "/users/search"} {
		t.Run(path, func(t *testing.T) {
			app := New(Config{}, Repositories{SessionAuth: &fakeSessionStore{}})

			res := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			app.ServeHTTP(res, req)

			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d -- GET %s must reach its own protected handler, not the public GET /users/{username} wildcard", res.Code, http.StatusUnauthorized, path)
			}
		})
	}
}

func TestHealthEndpoint(t *testing.T) {
	app := New(Config{}, Repositories{SessionAuth: &fakeSessionStore{}})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	app.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
}

func TestBackgroundHealthEndpointReportsPipelineSnapshot(t *testing.T) {
	monitor := pipeline.NewMonitor(time.Minute)
	monitor.Start("outbox-relay")
	monitor.Progress("outbox-relay", 2, "published")
	app := New(Config{Pipelines: monitor}, Repositories{SessionAuth: &fakeSessionStore{}})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/background", nil)
	app.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	body := res.Body.String()
	if !strings.Contains(body, `"name":"outbox-relay"`) || !strings.Contains(body, `"processed":2`) {
		t.Fatalf("body = %s, want pipeline progress", body)
	}
}

func TestReadinessEndpointChecksDependencyWithTimeout(t *testing.T) {
	store := &fakeSessionStore{}
	app := New(Config{Readiness: func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("readiness context has no deadline")
		}
		remaining := time.Until(deadline)
		if remaining <= 0 || remaining > 2*time.Second {
			t.Fatalf("readiness deadline remaining = %v", remaining)
		}
		return nil
	}}, Repositories{SessionAuth: store})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	app.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
	if store.refreshCalls != 0 {
		t.Fatal("readiness endpoint required authentication")
	}
}

func TestReadinessEndpointReportsDependencyFailure(t *testing.T) {
	app := New(Config{Readiness: func(context.Context) error {
		return errors.New("database unavailable")
	}}, Repositories{SessionAuth: &fakeSessionStore{}})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	app.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusServiceUnavailable)
	}
	if strings.TrimSpace(res.Body.String()) != `{"message":"Service unavailable"}` {
		t.Fatalf("body = %q", res.Body.String())
	}
}

func TestLogoutDoesNotRequireAuthenticatedSession(t *testing.T) {
	app := New(Config{}, Repositories{SessionAuth: &fakeSessionStore{}})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/sessions", nil)
	app.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
}

func TestNotFoundJSON(t *testing.T) {
	app := New(Config{}, Repositories{SessionAuth: &fakeSessionStore{
		refreshSession: httpx.Session{ID: "hashed-session-id", UserID: "1"},
	}})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "AAAAAAAAAAAAAAAAAAAAAAAAAAAA"})
	app.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	if strings.TrimSpace(res.Body.String()) != `{"message":"Not Found"}` {
		t.Fatalf("body = %q", res.Body.String())
	}
}

func TestOriginGuardRejectsCrossOriginStateChangingRequests(t *testing.T) {
	store := &fakeSessionStore{}
	app := New(Config{}, Repositories{SessionAuth: store})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(`{}`))
	req.Host = "localhost:8080"
	req.Header.Set("Origin", "http://evil.example")
	app.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
	}
	if store.refreshCalls != 0 {
		t.Fatalf("origin rejection should not hit store")
	}
}

func TestOriginGuardRejectsMalformedOrigins(t *testing.T) {
	app := New(Config{}, Repositories{SessionAuth: &fakeSessionStore{}})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(`{}`))
	req.Header.Set("Origin", "not a url")
	app.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
	}
}

func TestSessionMissingCookie(t *testing.T) {
	app := New(Config{}, Repositories{SessionAuth: &fakeSessionStore{}})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	app.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestSessionMalformedCookieClearsWithoutStore(t *testing.T) {
	store := &fakeSessionStore{}
	app := New(Config{}, Repositories{SessionAuth: store})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "not-a-valid-session"})
	app.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
	if store.refreshCalls != 0 {
		t.Fatalf("malformed session should not hit store")
	}
	if got := res.Header().Get("Set-Cookie"); !strings.Contains(got, "session=") || !strings.Contains(got, "Max-Age=0") {
		t.Fatalf("expected clearing Set-Cookie header, got %q", got)
	}
}

func TestSessionStoreErrorDoesNotClearCookie(t *testing.T) {
	app := New(Config{}, Repositories{SessionAuth: &fakeSessionStore{refreshErr: errors.New("database unavailable")}})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "AAAAAAAAAAAAAAAAAAAAAAAAAAAA"})
	app.ServeHTTP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusInternalServerError)
	}
	if got := res.Header().Get("Set-Cookie"); got != "" {
		t.Fatalf("server error should not clear cookie, got %q", got)
	}
}

func TestSessionInvalidClearsCookie(t *testing.T) {
	app := New(Config{}, Repositories{SessionAuth: &fakeSessionStore{}})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/images", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "AAAAAAAAAAAAAAAAAAAAAAAAAAAA"})
	app.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
	if got := res.Header().Get("Set-Cookie"); !strings.Contains(got, "session=") {
		t.Fatalf("expected clearing Set-Cookie header, got %q", got)
	}
}

func TestSessionValidRefreshesCookie(t *testing.T) {
	store := &fakeSessionStore{
		refreshSession: httpx.Session{ID: "hashed-session-id", UserID: "1"},
	}
	app := New(Config{}, Repositories{SessionAuth: store})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "AAAAAAAAAAAAAAAAAAAAAAAAAAAA"})
	app.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	if store.refreshCalls != 1 {
		t.Fatalf("refresh calls = %d, want 1", store.refreshCalls)
	}
	if got := res.Header().Get("Set-Cookie"); !strings.Contains(got, "session=AAAAAAAAAAAAAAAAAAAAAAAAAAAA") {
		t.Fatalf("expected refreshed Set-Cookie header, got %q", got)
	}
}
