package search

import "context"

type Repository interface {
	SearchUsers(ctx context.Context, q string) ([]UserResult, error)
	SearchHashtags(ctx context.Context, q string) ([]HashtagResult, error)
	// FollowingUsernames reports which of usernames the viewer follows, for
	// boosting blended search results by social-graph proximity.
	FollowingUsernames(ctx context.Context, viewerID string, usernames []string) (map[string]bool, error)

	// PostLikeCounts hydrates like_count by public ID at read time, since the
	// Meilisearch posts document doesn't carry it.
	PostLikeCounts(ctx context.Context, postIDs []string) (map[string]int, error)

	// RecordRecentSearch upserts (userID, entityType, reference), bumping an
	// existing entry's timestamp instead of duplicating it, then trims the
	// user's history back down to recentSearchLimit.
	RecordRecentSearch(ctx context.Context, userID, entityType, reference string) error
	// ListRecentSearches returns the user's history, newest first, silently
	// excluding any users/hashtags entry whose reference no longer resolves.
	ListRecentSearches(ctx context.Context, userID string) ([]RecentSearchItem, error)
	DeleteRecentSearch(ctx context.Context, userID, publicID string) error
	ClearRecentSearches(ctx context.Context, userID string) error
}
