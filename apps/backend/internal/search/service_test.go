package search

import (
	"context"
	"testing"
)

type fakeRepository struct {
	users           []UserResult
	hashtags        []HashtagResult
	usersContext    context.Context
	hashtagsContext context.Context
	usersQuery      string
	hashtagsQuery   string

	recentItems []RecentSearchItem
	recordCall  recordRecentSearchCall
	listUserID  string
	deleteCall  [2]string // userID, publicID
	clearUserID string
}

func (r *fakeRepository) SearchUsers(ctx context.Context, q string) ([]UserResult, error) {
	r.usersContext = ctx
	r.usersQuery = q
	return r.users, nil
}

func (r *fakeRepository) SearchHashtags(ctx context.Context, q string) ([]HashtagResult, error) {
	r.hashtagsContext = ctx
	r.hashtagsQuery = q
	return r.hashtags, nil
}

func (r *fakeRepository) FollowingUsernames(_ context.Context, _ string, _ []string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (r *fakeRepository) RecordRecentSearch(_ context.Context, userID, entityType, reference string) error {
	r.recordCall = recordRecentSearchCall{userID, entityType, reference}
	return nil
}

func (r *fakeRepository) ListRecentSearches(_ context.Context, userID string) ([]RecentSearchItem, error) {
	r.listUserID = userID
	return r.recentItems, nil
}

func (r *fakeRepository) DeleteRecentSearch(_ context.Context, userID, publicID string) error {
	r.deleteCall = [2]string{userID, publicID}
	return nil
}

func (r *fakeRepository) ClearRecentSearches(_ context.Context, userID string) error {
	r.clearUserID = userID
	return nil
}

func TestServiceDelegatesSearchesWithoutChangingResults(t *testing.T) {
	users := make([]UserResult, 9)
	for i := range users {
		users[i] = UserResult{Username: "user"}
	}
	hashtags := make([]HashtagResult, 9)
	for i := range hashtags {
		hashtags[i] = HashtagResult{Name: "tag", PostCount: i}
	}
	repository := &fakeRepository{users: users, hashtags: hashtags}
	service := NewService(repository)

	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("request"), "search")
	gotUsers, err := service.SearchUsers(ctx, "ali")
	if err != nil {
		t.Fatal(err)
	}
	gotHashtags, err := service.SearchHashtags(ctx, "cat")
	if err != nil {
		t.Fatal(err)
	}

	if len(gotUsers) != len(users) || len(gotHashtags) != len(hashtags) {
		t.Fatalf("result lengths = %d, %d; want %d, %d", len(gotUsers), len(gotHashtags), len(users), len(hashtags))
	}
	if repository.usersQuery != "ali" || repository.hashtagsQuery != "cat" {
		t.Fatalf("queries = %q, %q", repository.usersQuery, repository.hashtagsQuery)
	}
	if repository.usersContext != ctx || repository.hashtagsContext != ctx {
		t.Fatal("service did not pass request context to repository")
	}
}

func TestServiceDelegatesRecentSearches(t *testing.T) {
	want := []RecentSearchItem{{ID: "id-1", Type: "posts", Item: "cats"}}
	repository := &fakeRepository{recentItems: want}
	service := NewService(repository)
	ctx := context.Background()

	if err := service.RecordRecentSearch(ctx, "42", "users", "alice"); err != nil {
		t.Fatal(err)
	}
	if repository.recordCall != (recordRecentSearchCall{"42", "users", "alice"}) {
		t.Fatalf("recordCall = %+v", repository.recordCall)
	}

	got, err := service.ListRecentSearches(ctx, "42")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != want[0].ID || repository.listUserID != "42" {
		t.Fatalf("ListRecentSearches = %+v, userID = %q", got, repository.listUserID)
	}

	if err := service.DeleteRecentSearch(ctx, "42", "public-1"); err != nil {
		t.Fatal(err)
	}
	if repository.deleteCall != [2]string{"42", "public-1"} {
		t.Fatalf("deleteCall = %v", repository.deleteCall)
	}

	if err := service.ClearRecentSearches(ctx, "42"); err != nil {
		t.Fatal(err)
	}
	if repository.clearUserID != "42" {
		t.Fatalf("clearUserID = %q, want %q", repository.clearUserID, "42")
	}
}
