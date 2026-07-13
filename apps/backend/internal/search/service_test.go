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
