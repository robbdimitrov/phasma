package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRateLimitKeyUserID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/posts", nil)
	req = WithUserID(req, "42")
	policy := RateLimitPolicy{Name: "read"}

	key := rateLimitKey(req, policy)
	if key != "read:user:42" {
		t.Fatalf("key = %q, want %q", key, "read:user:42")
	}
}

func TestRateLimitKeySessionCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/sessions", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "tok123"})
	policy := RateLimitPolicy{Name: "strict"}

	key := rateLimitKey(req, policy)
	if key != "strict:session:tok123" {
		t.Fatalf("key = %q, want %q", key, "strict:session:tok123")
	}
}

func TestRateLimitKeyIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/sessions", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	policy := RateLimitPolicy{Name: "strict"}

	key := rateLimitKey(req, policy)
	if key != "strict:ip:1.2.3.4" {
		t.Fatalf("key = %q, want %q", key, "strict:ip:1.2.3.4")
	}
}

func TestRateLimitKeyUserIDTakesPrecedenceOverCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/posts", nil)
	req = WithUserID(req, "99")
	req.AddCookie(&http.Cookie{Name: "session", Value: "tok123"})
	policy := RateLimitPolicy{Name: "read"}

	key := rateLimitKey(req, policy)
	if key != "read:user:99" {
		t.Fatalf("key = %q, want %q", key, "read:user:99")
	}
}

func TestClientIPNoProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	req.Header.Set("X-Forwarded-For", "203.0.113.5")

	trustProxy = false
	ip := ClientIP(req)
	if ip != "10.0.0.1" {
		t.Fatalf("ClientIP = %q, want %q", ip, "10.0.0.1")
	}
}

func TestClientIPTrustProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.2")

	trustProxy = true
	defer func() { trustProxy = false }()
	ip := ClientIP(req)
	if ip != "203.0.113.5" {
		t.Fatalf("ClientIP = %q, want %q", ip, "203.0.113.5")
	}
}

func TestClientIPTrustProxyRejectsInvalidForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	req.Header.Set("X-Forwarded-For", "fake-ip, 203.0.113.5")

	trustProxy = true
	defer func() { trustProxy = false }()
	ip := ClientIP(req)
	if ip != "10.0.0.1" {
		t.Fatalf("ClientIP = %q, want fallback RemoteAddr IP", ip)
	}
}

func TestRateLimitAllowsExemptPath(t *testing.T) {
	handler := RateLimit(&denyAllStore{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, path := range []string{"/health", "/health/background", "/ready"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("%s status = %d, want %d", path, rec.Code, http.StatusNoContent)
		}
	}
}

func TestRateLimitPolicyUsesTypeaheadBucketForSearchEndpoints(t *testing.T) {
	for _, path := range []string{"/users/search", "/hashtags/search", "/search"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		policy, exempt := rateLimitPolicy(req)
		if exempt || policy.Name != "typeahead" || policy.Burst != 20 || policy.Rate != 5 {
			t.Fatalf("%s policy = %+v, exempt %v", path, policy, exempt)
		}
	}
}

func TestRateLimitBlocksWhenDenied(t *testing.T) {
	handler := RateLimit(&denyAllStore{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

type denyAllStore struct{}

func (denyAllStore) Allow(_ context.Context, _ string, _ RateLimitPolicy) (RateLimitDecision, error) {
	return RateLimitDecision{Allowed: false, RetryAfter: 5 * time.Second}, nil
}
