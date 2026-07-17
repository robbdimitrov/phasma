package search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// newTestSearchClient bypasses provisioning/settings so unit tests can call
// lower-level client methods directly.
func newTestSearchClient(baseURL string) *SearchClient {
	return &SearchClient{
		baseURL:    baseURL,
		scopedKey:  "test-key",
		httpClient: &http.Client{},
	}
}

func TestSearchClientDoJSONSetsAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestSearchClient(srv.URL)
	_ = c.doJSON(context.Background(), http.MethodGet, "/test", "my-key", nil, nil)

	if gotAuth != "Bearer my-key" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer my-key")
	}
}

func TestSearchClientDoJSONSetsContentTypeForBody(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestSearchClient(srv.URL)
	_ = c.doJSON(context.Background(), http.MethodPost, "/test", "key", map[string]string{"a": "b"}, nil)

	if gotCT != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", gotCT)
	}
}

func TestSearchClientDoJSONDecodesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key":"scoped-abc"}`))
	}))
	defer srv.Close()

	c := newTestSearchClient(srv.URL)
	var out struct {
		Key string `json:"key"`
	}
	if err := c.doJSON(context.Background(), http.MethodGet, "/keys/1", "master", nil, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Key != "scoped-abc" {
		t.Fatalf("Key = %q, want %q", out.Key, "scoped-abc")
	}
}

func TestSearchClientDoJSONReturnsErrorOnBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := newTestSearchClient(srv.URL)
	err := c.doJSON(context.Background(), http.MethodGet, "/test", "bad-key", nil, nil)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestSearchClientDoJSONErrorDoesNotLeakKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	secretKey := "super-secret-master-key"
	c := newTestSearchClient(srv.URL)
	err := c.doJSON(context.Background(), http.MethodPost, "/keys", secretKey, nil, nil)
	if err != nil && strings.Contains(err.Error(), secretKey) {
		t.Fatalf("error leaks key: %v", err)
	}
}

func TestSearchClientProvisionScopedKeyLimitsScopeAndExpiry(t *testing.T) {
	var payload struct {
		Actions     []string `json:"actions"`
		Indexes     []string `json:"indexes"`
		ExpiresAt   string   `json:"expiresAt"`
		Description string   `json:"description"`
	}
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/keys" || r.Method != http.MethodPost {
			t.Fatalf("request = %s %s, want POST /keys", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode key payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key":"scoped-abc"}`))
	}))
	defer srv.Close()

	before := time.Now().UTC().Add(searchScopedKeyTTL - time.Minute)
	c := newTestSearchClient(srv.URL)
	key, err := c.provisionScopedKey(context.Background(), "master-key")
	after := time.Now().UTC().Add(searchScopedKeyTTL + time.Minute)
	if err != nil {
		t.Fatalf("provisionScopedKey: %v", err)
	}
	if key != "scoped-abc" {
		t.Fatalf("key = %q, want scoped-abc", key)
	}
	if gotAuth != "Bearer master-key" {
		t.Fatalf("Authorization = %q, want Bearer master-key", gotAuth)
	}
	if !slices.Equal(payload.Actions, []string{"search", "documents.add", "documents.delete"}) {
		t.Fatalf("actions = %v", payload.Actions)
	}
	if !slices.Equal(payload.Indexes, []string{"users", "posts", "hashtags"}) {
		t.Fatalf("indexes = %v", payload.Indexes)
	}
	expiresAt, err := time.Parse(time.RFC3339, payload.ExpiresAt)
	if err != nil {
		t.Fatalf("expiresAt = %q is not RFC3339: %v", payload.ExpiresAt, err)
	}
	if expiresAt.Before(before) || expiresAt.After(after) {
		t.Fatalf("expiresAt = %v, want within %v and %v", expiresAt, before, after)
	}
	if payload.Description == "" {
		t.Fatal("description should be set")
	}
}

func TestSearchClientUpsertDocumentsSendsPrimaryKey(t *testing.T) {
	tests := []struct {
		index      string
		primaryKey string
	}{
		{index: "users", primaryKey: "id"},
		{index: "posts", primaryKey: "post_id"},
		{index: "hashtags", primaryKey: "name"},
	}

	for _, tt := range tests {
		t.Run(tt.index, func(t *testing.T) {
			var gotPath string
			var gotPrimaryKey string
			var gotMethod string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotPrimaryKey = r.URL.Query().Get("primaryKey")
				gotMethod = r.Method
				w.WriteHeader(http.StatusAccepted)
			}))
			defer srv.Close()

			c := newTestSearchClient(srv.URL)
			docs := []map[string]string{{tt.primaryKey: "1"}}
			if err := c.UpsertDocuments(context.Background(), tt.index, docs); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotPath != "/indexes/"+tt.index+"/documents" {
				t.Fatalf("path = %q, want /indexes/%s/documents", gotPath, tt.index)
			}
			if gotPrimaryKey != tt.primaryKey {
				t.Fatalf("primaryKey = %q, want %s", gotPrimaryKey, tt.primaryKey)
			}
			if gotMethod != http.MethodPost {
				t.Fatalf("method = %q, want POST", gotMethod)
			}
		})
	}
}

func TestSearchClientUpsertDocumentsRejectsUnknownIndex(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	c := newTestSearchClient(srv.URL)
	if err := c.UpsertDocuments(context.Background(), "comments", []map[string]string{{"id": "1"}}); err == nil {
		t.Fatal("expected error for unknown index")
	}
	if called {
		t.Fatal("server was called for unknown index")
	}
}

func TestSearchClientDeleteDocumentSendsCorrectPath(t *testing.T) {
	var gotPath string
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestSearchClient(srv.URL)
	if err := c.DeleteDocument(context.Background(), "posts", "post-42"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/indexes/posts/documents/post-42" {
		t.Fatalf("path = %q, want /indexes/posts/documents/post-42", gotPath)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("method = %q, want DELETE", gotMethod)
	}
}

func TestSearchClientSearchReturnsHits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":[{"name":"cats","post_count":5}],"limit":20}`))
	}))
	defer srv.Close()

	c := newTestSearchClient(srv.URL)
	result, err := c.Search(context.Background(), "hashtags", map[string]any{"q": "cat"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var hits []map[string]any
	if err := json.Unmarshal(result.Hits, &hits); err != nil {
		t.Fatalf("unmarshal hits: %v", err)
	}
	if len(hits) != 1 || hits[0]["name"] != "cats" {
		t.Fatalf("unexpected hits: %v", hits)
	}
}

func TestSearchClientApplySettingsSendsCorrectPaths(t *testing.T) {
	var paths []string
	var auth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		auth = append(auth, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	c := newTestSearchClient(srv.URL)
	if err := c.ApplySettings(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		"/indexes/users/settings",
		"/indexes/posts/settings",
		"/indexes/hashtags/settings",
	}
	if len(paths) != len(want) {
		t.Fatalf("got %d requests, want %d: %v", len(paths), len(want), paths)
	}
	for i, p := range paths {
		if p != want[i] {
			t.Fatalf("path[%d] = %q, want %q", i, p, want[i])
		}
		if auth[i] != "Bearer test-key" {
			t.Fatalf("auth[%d] = %q, want Bearer test-key", i, auth[i])
		}
	}
}

func TestSearchClientNewClientAppliesSettingsWithMasterThenStoresScopedKey(t *testing.T) {
	var settingsAuth []string
	var keyAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPatch && strings.HasSuffix(r.URL.Path, "/settings"):
			settingsAuth = append(settingsAuth, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusAccepted)
		case r.Method == http.MethodPost && r.URL.Path == "/keys":
			keyAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"key":"scoped-abc"}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	client, err := NewSearchClient(context.Background(), srv.URL, "master-key")
	if err != nil {
		t.Fatalf("NewSearchClient: %v", err)
	}
	if client.scopedKey != "scoped-abc" {
		t.Fatalf("scopedKey = %q, want scoped-abc", client.scopedKey)
	}
	if keyAuth != "Bearer master-key" {
		t.Fatalf("key auth = %q, want Bearer master-key", keyAuth)
	}
	if len(settingsAuth) != 3 {
		t.Fatalf("settings requests = %d, want 3", len(settingsAuth))
	}
	for i, got := range settingsAuth {
		if got != "Bearer master-key" {
			t.Fatalf("settings auth[%d] = %q, want Bearer master-key", i, got)
		}
	}
}

func TestSearchClientNewClientLive(t *testing.T) {
	searchURL := os.Getenv("MEILI_TEST_URL")
	if searchURL == "" {
		t.Skip("MEILI_TEST_URL not set; skipping live search backend test")
	}
	masterKey := os.Getenv("MEILI_TEST_MASTER_KEY")

	client, err := NewSearchClient(context.Background(), searchURL, masterKey)
	if err != nil {
		t.Fatalf("NewSearchClient: %v", err)
	}
	if client.scopedKey == "" {
		t.Fatal("scoped key is empty after construction")
	}
}
