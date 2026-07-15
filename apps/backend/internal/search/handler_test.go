package search

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"phasma/backend/internal/httpx"
	"phasma/backend/internal/store"
)

type recordRecentSearchCall struct {
	userID, entityType, reference string
}

type fakeApplication struct {
	users          []UserResult
	hashtags       []HashtagResult
	usersQuery     string
	hashtagsQuery  string
	usersCalled    bool
	hashtagsCalled bool
	following      map[string]bool
	followingCalls [][]string

	recentItems    []RecentSearchItem
	recentErr      error
	recordCalls    []recordRecentSearchCall
	listCalls      []string
	deleteCalls    []string
	deletePublicID string
	clearCalls     []string
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

func (a *fakeApplication) FollowingUsernames(_ context.Context, _ string, usernames []string) (map[string]bool, error) {
	a.followingCalls = append(a.followingCalls, usernames)
	if a.following == nil {
		return map[string]bool{}, nil
	}
	return a.following, nil
}

func (a *fakeApplication) RecordRecentSearch(_ context.Context, userID, entityType, reference string) error {
	a.recordCalls = append(a.recordCalls, recordRecentSearchCall{userID, entityType, reference})
	return a.recentErr
}

func (a *fakeApplication) ListRecentSearches(_ context.Context, userID string) ([]RecentSearchItem, error) {
	a.listCalls = append(a.listCalls, userID)
	if a.recentErr != nil {
		return nil, a.recentErr
	}
	return a.recentItems, nil
}

func (a *fakeApplication) DeleteRecentSearch(_ context.Context, userID, publicID string) error {
	a.deleteCalls = append(a.deleteCalls, userID)
	a.deletePublicID = publicID
	return a.recentErr
}

func (a *fakeApplication) ClearRecentSearches(_ context.Context, userID string) error {
	a.clearCalls = append(a.clearCalls, userID)
	return a.recentErr
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

func TestResolvePostsQuery(t *testing.T) {
	tests := []struct {
		name       string
		q          string
		wantQ      string
		wantFilter string
		wantOK     bool
	}{
		{name: "plain text", q: "sunset", wantQ: "sunset", wantFilter: "", wantOK: true},
		{name: "valid hashtag", q: "#vacation", wantQ: "", wantFilter: `hashtags = "vacation"`, wantOK: true},
		{name: "invalid hashtag", q: "#not valid!", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQ, gotFilter, ok := resolvePostsQuery(tt.q)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if gotQ != tt.wantQ || gotFilter != tt.wantFilter {
				t.Fatalf("got (%q, %q), want (%q, %q)", gotQ, gotFilter, tt.wantQ, tt.wantFilter)
			}
		})
	}
}

func newFakeMeiliMultiClient(t *testing.T, byIndex map[string]string) *SearchClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		for index, body := range byIndex {
			if strings.Contains(r.URL.Path, "/indexes/"+index+"/search") {
				_, _ = w.Write([]byte(body))
				return
			}
		}
		_, _ = w.Write([]byte(`{"hits":[]}`))
	}))
	t.Cleanup(server.Close)
	return &SearchClient{baseURL: server.URL, scopedKey: "test-key", httpClient: server.Client()}
}

// repeatHits builds a Meilisearch-shaped {"hits":[...]} body with n copies of
// hit repeated, for tests that only need a type to have "enough" results and
// don't care about their content.
func repeatHits(hit string, n int) string {
	hits := make([]string, n)
	for i := range hits {
		hits[i] = hit
	}
	return `{"hits":[` + strings.Join(hits, ",") + `]}`
}

func TestSearchAllRequiresMeilisearchClient(t *testing.T) {
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=cats&type=all", nil)

	Handler{Service: &fakeApplication{}}.Search(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusServiceUnavailable)
	}
}

func TestSearchAllReturnsBlendedItemsTaggedByType(t *testing.T) {
	client := newFakeMeiliMultiClient(t, map[string]string{
		"users":    `{"hits":[{"username":"alice","name":"Alice"}]}`,
		"posts":    `{"hits":[{"post_id":"p1","username":"alice","description":"hi","filename":"a.jpg"}]}`,
		"hashtags": `{"hits":[{"name":"cats","post_count":4}]}`,
	})
	application := &fakeApplication{}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=cats&type=all&limit=5", nil)

	Handler{Client: client, Service: application}.Search(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", res.Code, http.StatusOK, res.Body.String())
	}
	var body struct {
		Items []struct {
			Type string          `json:"type"`
			Item json.RawMessage `json:"item"`
		} `json:"items"`
		NextCursor *string `json:"nextCursor"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var sawUsers, sawPosts, sawHashtags bool
	for _, item := range body.Items {
		switch item.Type {
		case "users":
			sawUsers = true
		case "posts":
			sawPosts = true
		case "hashtags":
			sawHashtags = true
		default:
			t.Fatalf("unexpected item type %q", item.Type)
		}
	}
	if !sawUsers || !sawPosts || !sawHashtags {
		t.Fatalf("expected all three types present, got users=%v posts=%v hashtags=%v", sawUsers, sawPosts, sawHashtags)
	}
}

func TestSearchAllBoostsFollowedUsersWithinThePageOnly(t *testing.T) {
	// limit=10 -> targets (2 users, 6 posts, 2 hashtags). Posts and hashtags
	// are given a full target+1 so neither is short, isolating this test to
	// the users follow-boost/frozen-prefix behavior with no cross-type
	// backfill in play.
	client := newFakeMeiliMultiClient(t, map[string]string{
		"users":    `{"hits":[{"username":"alice","name":"Alice"},{"username":"bob","name":"Bob"},{"username":"carol","name":"Carol"}]}`,
		"posts":    repeatHits(`{"post_id":"p","username":"u","description":"d","filename":"f.jpg"}`, 7),
		"hashtags": repeatHits(`{"name":"h"}`, 3),
	})
	application := &fakeApplication{following: map[string]bool{"bob": true}}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=al&type=all&limit=10", nil)
	req = httpx.WithUserID(req, "42")

	Handler{Client: client, Service: application}.Search(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", res.Code, http.StatusOK, res.Body.String())
	}
	var body struct {
		Items []struct {
			Type string `json:"type"`
			Item struct {
				Username string `json:"username"`
			} `json:"item"`
		} `json:"items"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var usernames []string
	for _, item := range body.Items {
		if item.Type == "users" {
			usernames = append(usernames, item.Item.Username)
		}
	}
	// carol is the lookahead (3rd fetched, beyond the 2-item target) and
	// must never appear: promoting her via the follow-boost partition would
	// duplicate/skip results across pages.
	if got := strings.Join(usernames, ","); got != "bob,alice" {
		t.Fatalf("users order = %q, want %q (bob boosted first, carol held back as lookahead)", got, "bob,alice")
	}
	if len(application.followingCalls) != 1 || strings.Join(application.followingCalls[0], ",") != "alice,bob" {
		t.Fatalf("FollowingUsernames calls = %v, want one call with [alice bob]", application.followingCalls)
	}
}

func TestSearchAllAppliesHashtagFilterToPosts(t *testing.T) {
	var postsBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/indexes/posts/search") {
			postsBody, _ = io.ReadAll(r.Body)
		}
		_, _ = w.Write([]byte(`{"hits":[]}`))
	}))
	t.Cleanup(server.Close)
	client := &SearchClient{baseURL: server.URL, scopedKey: "test-key", httpClient: server.Client()}

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=%23vacation&type=all&limit=10", nil)

	Handler{Client: client, Service: &fakeApplication{}}.Search(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", res.Code, http.StatusOK, res.Body.String())
	}
	var sent struct {
		Q      string `json:"q"`
		Filter string `json:"filter"`
	}
	if err := json.Unmarshal(postsBody, &sent); err != nil {
		t.Fatalf("decode posts request body: %v", err)
	}
	if sent.Q != "" || sent.Filter != `hashtags = "vacation"` {
		t.Fatalf("posts request = %+v, want empty q and hashtag filter", sent)
	}
}

func TestSearchAllRejectsInvalidHashtag(t *testing.T) {
	client := newFakeMeiliMultiClient(t, map[string]string{})
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=%23not+valid%21&type=all", nil)

	Handler{Client: client, Service: &fakeApplication{}}.Search(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func TestRecordRecentSearchRequiresSession(t *testing.T) {
	application := &fakeApplication{}
	res := httptest.NewRecorder()
	body := strings.NewReader(`{"type":"users","reference":"alice"}`)
	req := httptest.NewRequest(http.MethodPost, "/search/recent", body)

	Handler{Service: application}.RecordRecentSearch(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
	if len(application.recordCalls) != 0 {
		t.Fatal("unauthenticated request reached service")
	}
}

func TestRecordRecentSearchValidatesTypeAndReference(t *testing.T) {
	tests := []struct {
		name      string
		entType   string
		reference string
	}{
		{name: "unknown type", entType: "posts_of_users", reference: "alice"},
		{name: "users invalid username", entType: "users", reference: "not a username!"},
		{name: "hashtags invalid name", entType: "hashtags", reference: "not valid!"},
		{name: "posts empty", entType: "posts", reference: ""},
		{name: "posts whitespace-only", entType: "posts", reference: "   "},
		{name: "posts too long", entType: "posts", reference: strings.Repeat("界", 51)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			application := &fakeApplication{}
			res := httptest.NewRecorder()
			payload, _ := json.Marshal(recordRecentSearchRequest{Type: tt.entType, Reference: tt.reference})
			req := httptest.NewRequest(http.MethodPost, "/search/recent", strings.NewReader(string(payload)))
			req = httpx.WithUserID(req, "42")

			Handler{Service: application}.RecordRecentSearch(res, req)

			if res.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
			}
			if len(application.recordCalls) != 0 {
				t.Fatal("invalid input reached service")
			}
		})
	}
}

func TestRecordRecentSearchDelegatesValidInput(t *testing.T) {
	tests := []struct {
		name      string
		entType   string
		reference string
	}{
		{name: "users", entType: "users", reference: "alice"},
		{name: "hashtags", entType: "hashtags", reference: "cats"},
		{name: "posts", entType: "posts", reference: "sunset beach"},
		{name: "posts with hashtag prefix", entType: "posts", reference: "#vacation"},
		{name: "posts with user prefix", entType: "posts", reference: "@bob"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			application := &fakeApplication{}
			res := httptest.NewRecorder()
			payload, _ := json.Marshal(recordRecentSearchRequest{Type: tt.entType, Reference: tt.reference})
			req := httptest.NewRequest(http.MethodPost, "/search/recent", strings.NewReader(string(payload)))
			req = httpx.WithUserID(req, "42")

			Handler{Service: application}.RecordRecentSearch(res, req)

			if res.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d, body = %s", res.Code, http.StatusNoContent, res.Body.String())
			}
			if len(application.recordCalls) != 1 {
				t.Fatalf("recordCalls = %d, want 1", len(application.recordCalls))
			}
			got := application.recordCalls[0]
			if got.userID != "42" || got.entityType != tt.entType || got.reference != tt.reference {
				t.Fatalf("call = %+v, want {42 %s %s}", got, tt.entType, tt.reference)
			}
		})
	}
}

func TestRecordRecentSearchTrimsWhitespaceBeforeValidatingAndStoring(t *testing.T) {
	application := &fakeApplication{}
	res := httptest.NewRecorder()
	payload, _ := json.Marshal(recordRecentSearchRequest{Type: "  posts  ", Reference: "  cats  "})
	req := httptest.NewRequest(http.MethodPost, "/search/recent", strings.NewReader(string(payload)))
	req = httpx.WithUserID(req, "42")

	Handler{Service: application}.RecordRecentSearch(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body = %s", res.Code, http.StatusNoContent, res.Body.String())
	}
	if len(application.recordCalls) != 1 {
		t.Fatalf("recordCalls = %d, want 1", len(application.recordCalls))
	}
	got := application.recordCalls[0]
	if got.entityType != "posts" || got.reference != "cats" {
		t.Fatalf("call = %+v, want entityType \"posts\" reference \"cats\"", got)
	}
}

func TestListRecentSearchesRequiresSession(t *testing.T) {
	application := &fakeApplication{}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search/recent", nil)

	Handler{Service: application}.ListRecentSearches(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestListRecentSearchesReturnsBlendedItemShape(t *testing.T) {
	application := &fakeApplication{recentItems: []RecentSearchItem{
		{ID: "id-1", Type: "users", Item: UserResult{Username: "alice", Name: "Alice"}},
		{ID: "id-2", Type: "hashtags", Item: HashtagResult{Name: "cats", PostCount: 4}},
		{ID: "id-3", Type: "posts", Item: "sunset beach"},
	}}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search/recent", nil)
	req = httpx.WithUserID(req, "42")

	Handler{Service: application}.ListRecentSearches(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	var items []struct {
		ID   string          `json:"id"`
		Type string          `json:"type"`
		Item json.RawMessage `json:"item"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3", len(items))
	}
	if items[2].Type != "posts" || string(items[2].Item) != `"sunset beach"` {
		t.Fatalf("posts item = %+v", items[2])
	}
	if application.listCalls[0] != "42" {
		t.Fatalf("listCalls = %v, want [42]", application.listCalls)
	}
}

func TestDeleteRecentSearchValidatesID(t *testing.T) {
	application := &fakeApplication{}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/search/recent/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	req = httpx.WithUserID(req, "42")

	Handler{Service: application}.DeleteRecentSearch(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
	if len(application.deleteCalls) != 0 {
		t.Fatal("invalid id reached service")
	}
}

func TestDeleteRecentSearchEnforcesOwnership(t *testing.T) {
	const otherUsersID = "01904d2e-7f4d-7c33-ae21-2f94737eaa10"
	application := &fakeApplication{recentErr: store.ErrNotFound}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/search/recent/"+otherUsersID, nil)
	req.SetPathValue("id", otherUsersID)
	req = httpx.WithUserID(req, "42")

	Handler{Service: application}.DeleteRecentSearch(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	if application.deletePublicID != otherUsersID {
		t.Fatalf("deletePublicID = %q, want %q", application.deletePublicID, otherUsersID)
	}
}

func TestDeleteRecentSearchSucceeds(t *testing.T) {
	const id = "01904d2e-7f4d-7c33-ae21-2f94737eaa10"
	application := &fakeApplication{}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/search/recent/"+id, nil)
	req.SetPathValue("id", id)
	req = httpx.WithUserID(req, "42")

	Handler{Service: application}.DeleteRecentSearch(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
	if len(application.deleteCalls) != 1 || application.deleteCalls[0] != "42" {
		t.Fatalf("deleteCalls = %v, want [42]", application.deleteCalls)
	}
}

func TestClearRecentSearchesRequiresSession(t *testing.T) {
	application := &fakeApplication{}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/search/recent", nil)

	Handler{Service: application}.ClearRecentSearches(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestClearRecentSearchesSucceeds(t *testing.T) {
	application := &fakeApplication{}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/search/recent", nil)
	req = httpx.WithUserID(req, "42")

	Handler{Service: application}.ClearRecentSearches(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
	if len(application.clearCalls) != 1 || application.clearCalls[0] != "42" {
		t.Fatalf("clearCalls = %v, want [42]", application.clearCalls)
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
