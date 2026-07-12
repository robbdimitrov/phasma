package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeApplication struct {
	users          []UserResult
	hashtags       []HashtagResult
	usersQuery     string
	hashtagsQuery  string
	usersCalled    bool
	hashtagsCalled bool
}

func (a *fakeApplication) SearchUsers(_ context.Context, q string) ([]UserResult, error) {
	a.usersCalled = true
	a.usersQuery = q
	return a.users, nil
}

func (a *fakeApplication) SearchHashtags(_ context.Context, q string) ([]HashtagResult, error) {
	a.hashtagsCalled = true
	a.hashtagsQuery = q
	return a.hashtags, nil
}

func TestSearchQueryValidation(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		handle func(Handler, http.ResponseWriter, *http.Request)
	}{
		{
			name: "users empty",
			path: "/users/search",
			handle: func(handler Handler, w http.ResponseWriter, r *http.Request) {
				handler.SearchUsers(w, r)
			},
		},
		{
			name: "users whitespace",
			path: "/users/search?q=%20%09%20",
			handle: func(handler Handler, w http.ResponseWriter, r *http.Request) {
				handler.SearchUsers(w, r)
			},
		},
		{
			name: "users 51 Unicode characters",
			path: "/users/search?q=" + strings.Repeat("界", 51),
			handle: func(handler Handler, w http.ResponseWriter, r *http.Request) {
				handler.SearchUsers(w, r)
			},
		},
		{
			name: "hashtags empty",
			path: "/hashtags/search",
			handle: func(handler Handler, w http.ResponseWriter, r *http.Request) {
				handler.SearchHashtags(w, r)
			},
		},
		{
			name: "hashtags whitespace",
			path: "/hashtags/search?q=%20%09%20",
			handle: func(handler Handler, w http.ResponseWriter, r *http.Request) {
				handler.SearchHashtags(w, r)
			},
		},
		{
			name: "hashtags 51 Unicode characters",
			path: "/hashtags/search?q=" + strings.Repeat("界", 51),
			handle: func(handler Handler, w http.ResponseWriter, r *http.Request) {
				handler.SearchHashtags(w, r)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			application := &fakeApplication{}
			res := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, test.path, nil)

			test.handle(Handler{Service: application}, res, req)

			if res.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
			}
			if !strings.Contains(res.Body.String(), "Query must be 1 to 50 characters.") {
				t.Fatalf("body = %q", res.Body.String())
			}
			if application.usersCalled || application.hashtagsCalled {
				t.Fatal("invalid query reached service")
			}
		})
	}
}

func TestSearchUsersReturnsMinimalJSONShape(t *testing.T) {
	avatar := "avatar.jpg"
	application := &fakeApplication{users: []UserResult{
		{Username: "alice", Name: "Alice", Avatar: &avatar},
		{Username: "bob", Name: "Bob"},
	}}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/search?q=%20ali%20", nil)

	Handler{Service: application}.SearchUsers(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if got := strings.TrimSpace(res.Body.String()); got != `[{"username":"alice","name":"Alice","avatar":"avatar.jpg"},{"username":"bob","name":"Bob","avatar":null}]` {
		t.Fatalf("body = %q", got)
	}
	if application.usersQuery != "ali" {
		t.Fatalf("query = %q, want %q", application.usersQuery, "ali")
	}
}

func newFakeMeiliClient(t *testing.T, body string) *SearchClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return &SearchClient{baseURL: server.URL, scopedKey: "test-key", httpClient: server.Client()}
}

func TestSearchUsersFullSearchIncludesNameAndAvatar(t *testing.T) {
	client := newFakeMeiliClient(t, `{"hits":[{"username":"alice","name":"Alice","avatar":"avatar.jpg"}]}`)
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=ali&type=users", nil)

	Handler{Client: client}.Search(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if !strings.Contains(res.Body.String(), `"name":"Alice"`) || !strings.Contains(res.Body.String(), `"avatar":"avatar.jpg"`) {
		t.Fatalf("body = %q", res.Body.String())
	}
}

func TestSearchPostsFullSearchIncludesFilename(t *testing.T) {
	client := newFakeMeiliClient(t, `{"hits":[{"post_id":"post-1","username":"alice","description":"hi","filename":"photo.jpg"}]}`)
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=hi&type=posts", nil)

	Handler{Client: client}.Search(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if !strings.Contains(res.Body.String(), `"filename":"photo.jpg"`) {
		t.Fatalf("body = %q", res.Body.String())
	}
}

func TestSearchHashtagsReturnsMinimalJSONShape(t *testing.T) {
	application := &fakeApplication{hashtags: []HashtagResult{
		{Name: "cats", PostCount: 12},
		{Name: "catsofinstagram", PostCount: 3},
	}}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/hashtags/search?q=%20cat%20", nil)

	Handler{Service: application}.SearchHashtags(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if got := strings.TrimSpace(res.Body.String()); got != `[{"name":"cats","postCount":12},{"name":"catsofinstagram","postCount":3}]` {
		t.Fatalf("body = %q", got)
	}
	if application.hashtagsQuery != "cat" {
		t.Fatalf("query = %q, want %q", application.hashtagsQuery, "cat")
	}
}
