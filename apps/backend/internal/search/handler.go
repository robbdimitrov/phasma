package search

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"phasma/backend/internal/httpx"
	"phasma/backend/internal/pagination"

	"golang.org/x/sync/errgroup"
)

// hashtagNameRe matches valid hashtag names as stored in the database.
// Using this against user input prevents filter injection into search backend.
var hashtagNameRe = regexp.MustCompile(`^[A-Za-z0-9_]{1,50}$`)

const (
	maxQueryLen  = 50
	typeaheadLen = 8
)

type Application interface {
	SearchUsers(ctx context.Context, q string) ([]UserResult, error)
	SearchHashtags(ctx context.Context, q string) ([]HashtagResult, error)
	FollowingUsernames(ctx context.Context, viewerID string, usernames []string) (map[string]bool, error)
}

type Handler struct {
	Service Application
	Client  *SearchClient
}

func (h Handler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if !validQuery(q) {
		httpx.WriteMessage(w, http.StatusBadRequest, "Query must be 1 to 50 characters.")
		return
	}
	if h.Client != nil {
		results, err := searchUsersWithClient(r.Context(), h.Client, q)
		if err != nil {
			httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, results)
		return
	}
	results, err := h.Service.SearchUsers(r.Context(), q)
	if err != nil {
		httpx.WriteStoreError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, results)
}

func (h Handler) SearchHashtags(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if !validQuery(q) {
		httpx.WriteMessage(w, http.StatusBadRequest, "Query must be 1 to 50 characters.")
		return
	}
	if h.Client != nil {
		results, err := searchHashtagsWithClient(r.Context(), h.Client, q)
		if err != nil {
			httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, results)
		return
	}
	results, err := h.Service.SearchHashtags(r.Context(), q)
	if err != nil {
		httpx.WriteStoreError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, results)
}

// Search requires search backend; returns 503 when not configured.
func (h Handler) Search(w http.ResponseWriter, r *http.Request) {
	if h.Client == nil {
		httpx.WriteMessage(w, http.StatusServiceUnavailable, "Search service unavailable.")
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if !validQuery(q) {
		httpx.WriteMessage(w, http.StatusBadRequest, "Query must be 1 to 50 characters.")
		return
	}

	searchType := r.URL.Query().Get("type")
	switch searchType {
	case "users", "posts", "hashtags", "all":
	default:
		httpx.WriteMessage(w, http.StatusBadRequest, "type must be one of: users, posts, hashtags, all.")
		return
	}

	limit, ok := pagination.ParseLimit(r.URL.Query(), 20)
	if !ok {
		httpx.WriteMessage(w, http.StatusBadRequest, "Invalid limit.")
		return
	}

	if searchType == "all" {
		h.searchAll(w, r, q, limit)
		return
	}

	offset, err := decodeCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "Invalid cursor.")
		return
	}

	ctx := r.Context()
	var items any
	var count int

	switch searchType {
	case "users":
		hits, err := searchIndex(ctx, h.Client, "users", q, "", offset, limit)
		if err != nil {
			httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
			return
		}
		users := usersFromHits(hits)
		items = users
		count = len(users)

	case "hashtags":
		hits, err := searchIndex(ctx, h.Client, "hashtags", q, "", offset, limit)
		if err != nil {
			httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
			return
		}
		hashtags := hashtagsFromHits(hits)
		items = hashtags
		count = len(hashtags)

	case "posts":
		searchQ, filter, ok := resolvePostsQuery(q)
		if !ok {
			httpx.WriteMessage(w, http.StatusBadRequest, "Invalid hashtag.")
			return
		}
		hits, err := searchIndex(ctx, h.Client, "posts", searchQ, filter, offset, limit)
		if err != nil {
			httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
			return
		}
		posts := postsFromHits(hits)
		items = posts
		count = len(posts)
	}

	var nextCursor *string
	if count >= limit {
		nc := encodeCursor(offset + limit)
		nextCursor = &nc
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"items":      items,
		"nextCursor": nextCursor,
	})
}

// searchAll serves type=all: a single blended, ranked page drawn from all
// three indexes. See blend.go for the pagination/backfill invariants.
func (h Handler) searchAll(w http.ResponseWriter, r *http.Request, q string, limit int) {
	cur, err := decodeBlendCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "Invalid cursor.")
		return
	}

	postsQ, postsFilter, ok := resolvePostsQuery(q)
	if !ok {
		httpx.WriteMessage(w, http.StatusBadRequest, "Invalid hashtag.")
		return
	}

	targetUsers, targetPosts, targetHashtags := computeBlendTargets(limit)

	var userHits, postHits, hashtagHits []map[string]any
	group, ctx := errgroup.WithContext(r.Context())
	group.Go(func() error {
		hits, err := searchIndex(ctx, h.Client, "users", q, "", cur.Users, targetUsers+1)
		userHits = hits
		return err
	})
	group.Go(func() error {
		hits, err := searchIndex(ctx, h.Client, "posts", postsQ, postsFilter, cur.Posts, targetPosts+1)
		postHits = hits
		return err
	})
	group.Go(func() error {
		hits, err := searchIndex(ctx, h.Client, "hashtags", q, "", cur.Hashtags, targetHashtags+1)
		hashtagHits = hits
		return err
	})
	if err := group.Wait(); err != nil {
		httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	users := usersFromHits(userHits)
	posts := postsFromHits(postHits)
	hashtags := hashtagsFromHits(hashtagHits)

	plan := planBlend(targetUsers, targetPosts, targetHashtags, len(users), len(posts), len(hashtags))
	users = users[:plan.ConsumeUsers]
	posts = posts[:plan.ConsumePosts]
	hashtags = hashtags[:plan.ConsumeHashtags]

	if viewerID, ok := httpx.UserID(r); ok && len(users) > 0 {
		usernames := make([]string, len(users))
		for i, u := range users {
			usernames[i] = u.Username
		}
		if following, err := h.Service.FollowingUsernames(r.Context(), viewerID, usernames); err == nil {
			users = partitionByFollowing(users, following)
		}
	}

	var nextCursor *string
	if plan.HasMoreUsers || plan.HasMorePosts || plan.HasMoreHashtags {
		nc := encodeBlendCursor(blendCursor{
			Users:    cur.Users + plan.ConsumeUsers,
			Posts:    cur.Posts + plan.ConsumePosts,
			Hashtags: cur.Hashtags + plan.ConsumeHashtags,
		})
		nextCursor = &nc
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"items":      interleaveBlended(users, posts, hashtags),
		"nextCursor": nextCursor,
	})
}

func searchUsersWithClient(ctx context.Context, mc *SearchClient, q string) ([]UserResult, error) {
	hits, err := searchIndex(ctx, mc, "users", q, "", 0, typeaheadLen)
	if err != nil {
		return nil, err
	}
	return usersFromHits(hits), nil
}

func searchHashtagsWithClient(ctx context.Context, mc *SearchClient, q string) ([]HashtagResult, error) {
	hits, err := searchIndex(ctx, mc, "hashtags", q, "", 0, typeaheadLen)
	if err != nil {
		return nil, err
	}
	return hashtagsFromHits(hits), nil
}

func usersFromHits(hits []map[string]any) []UserResult {
	results := make([]UserResult, 0, len(hits))
	for _, hit := range hits {
		u := UserResult{Username: stringField(hit, "username"), Name: stringField(hit, "name")}
		if av, ok := hit["avatar"].(string); ok {
			u.Avatar = &av
		}
		results = append(results, u)
	}
	return results
}

func postsFromHits(hits []map[string]any) []PostResult {
	posts := make([]PostResult, 0, len(hits))
	for _, hit := range hits {
		posts = append(posts, PostResult{
			ID:          stringField(hit, "post_id"),
			Username:    stringField(hit, "username"),
			Description: stringField(hit, "description"),
			Filename:    stringField(hit, "filename"),
		})
	}
	return posts
}

func hashtagsFromHits(hits []map[string]any) []HashtagResult {
	results := make([]HashtagResult, 0, len(hits))
	for _, hit := range hits {
		results = append(results, HashtagResult{
			Name:      stringField(hit, "name"),
			PostCount: intField(hit, "post_count"),
		})
	}
	return results
}

func searchIndex(ctx context.Context, mc *SearchClient, index, q, filter string, offset, limit int) ([]map[string]any, error) {
	params := map[string]any{
		"q":      q,
		"offset": offset,
		"limit":  limit,
	}
	if filter != "" {
		params["filter"] = filter
	}
	result, err := mc.Search(ctx, index, params)
	if err != nil {
		return nil, err
	}
	var hits []map[string]any
	if err := json.Unmarshal(result.Hits, &hits); err != nil {
		return nil, fmt.Errorf("search: decode hits: %w", err)
	}
	return hits, nil
}

func encodeCursor(offset int) string {
	return base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

// An empty cursor returns offset 0 (first page).
func decodeCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	b, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(string(b))
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid cursor value")
	}
	return n, nil
}

func stringField(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// search backend returns JSON numbers as float64.
func intField(m map[string]any, key string) int {
	v, _ := m[key].(float64)
	return int(v)
}

// resolvePostsQuery splits a posts query into a Meilisearch search string and
// an optional exact-match hashtag filter. A "#tag" query filters by hashtag
// instead of doing a literal text search; ok is false for an invalid tag.
func resolvePostsQuery(q string) (searchQ, filter string, ok bool) {
	if !strings.HasPrefix(q, "#") {
		return q, "", true
	}
	tag := q[1:]
	if !hashtagNameRe.MatchString(tag) {
		return "", "", false
	}
	return "", fmt.Sprintf(`hashtags = "%s"`, tag), true
}

func validQuery(q string) bool {
	n := utf8.RuneCountInString(q)
	return n >= 1 && n <= maxQueryLen
}
