package search

import "context"

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) SearchUsers(ctx context.Context, q string) ([]UserResult, error) {
	return s.repository.SearchUsers(ctx, q)
}

func (s *Service) SearchHashtags(ctx context.Context, q string) ([]HashtagResult, error) {
	return s.repository.SearchHashtags(ctx, q)
}

func (s *Service) FollowingUsernames(ctx context.Context, viewerID string, usernames []string) (map[string]bool, error) {
	return s.repository.FollowingUsernames(ctx, viewerID, usernames)
}

func (s *Service) PostLikeCounts(ctx context.Context, postIDs []string) (map[string]int, error) {
	return s.repository.PostLikeCounts(ctx, postIDs)
}

func (s *Service) RecordRecentSearch(ctx context.Context, userID, entityType, reference string) error {
	return s.repository.RecordRecentSearch(ctx, userID, entityType, reference)
}

func (s *Service) ListRecentSearches(ctx context.Context, userID string) ([]RecentSearchItem, error) {
	return s.repository.ListRecentSearches(ctx, userID)
}

func (s *Service) DeleteRecentSearch(ctx context.Context, userID, publicID string) error {
	return s.repository.DeleteRecentSearch(ctx, userID, publicID)
}

func (s *Service) ClearRecentSearches(ctx context.Context, userID string) error {
	return s.repository.ClearRecentSearches(ctx, userID)
}
