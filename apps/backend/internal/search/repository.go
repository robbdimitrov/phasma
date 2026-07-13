package search

import "context"

type Repository interface {
	SearchUsers(ctx context.Context, q string) ([]UserResult, error)
	SearchHashtags(ctx context.Context, q string) ([]HashtagResult, error)
	// FollowingUsernames reports which of usernames the viewer follows, for
	// boosting blended search results by social-graph proximity.
	FollowingUsernames(ctx context.Context, viewerID string, usernames []string) (map[string]bool, error)
}
