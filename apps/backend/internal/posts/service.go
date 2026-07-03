package posts

import (
	"context"

	"phasma/backend/internal/pagination"
)

type CreatePostCommand struct {
	UserID      string
	Filename    string
	Description *string
}

type CreatePostResult struct {
	PublicID string
	Created  bool
}

type ListQuery struct {
	Username      string
	Cursor        *pagination.Cursor
	Limit         int
	CurrentUserID string
}

type DeletePostCommand struct {
	PublicID string
	UserID   string
}

type Repository interface {
	CreatePost(ctx context.Context, userID, filename string, description *string, tags []string) (string, bool, error)
	GetPosts(ctx context.Context, username string, cursor *pagination.Cursor, limit int, currentUserID string) ([]Post, *pagination.Cursor, error)
	GetLikedPosts(ctx context.Context, username string, cursor *pagination.Cursor, limit int, currentUserID string) ([]Post, *pagination.Cursor, error)
	GetPost(ctx context.Context, publicID, currentUserID string) (Post, bool, error)
	DeletePost(ctx context.Context, publicID, userID string) (string, error)
	LikePost(ctx context.Context, publicID, userID string) error
	UnlikePost(ctx context.Context, publicID, userID string) error
	ListPopularPosts(ctx context.Context, viewerID string, limit int) ([]Post, error)
}

type FileRepository interface {
	Delete(filename string)
}

type Service struct {
	repository Repository
	files      FileRepository
}

func NewService(repository Repository, files FileRepository) *Service {
	return &Service{repository: repository, files: files}
}

func (s *Service) CreatePost(ctx context.Context, command CreatePostCommand) (CreatePostResult, error) {
	var tags []string
	if command.Description != nil {
		tags = ExtractHashtags(*command.Description)
	}
	publicID, created, err := s.repository.CreatePost(
		ctx, command.UserID, command.Filename, command.Description, tags,
	)
	return CreatePostResult{PublicID: publicID, Created: created}, err
}

func (s *Service) GetPosts(ctx context.Context, query ListQuery) ([]Post, *pagination.Cursor, error) {
	return s.repository.GetPosts(
		ctx, query.Username, query.Cursor, query.Limit, query.CurrentUserID,
	)
}

func (s *Service) GetLikedPosts(ctx context.Context, query ListQuery) ([]Post, *pagination.Cursor, error) {
	return s.repository.GetLikedPosts(
		ctx, query.Username, query.Cursor, query.Limit, query.CurrentUserID,
	)
}

func (s *Service) GetPost(ctx context.Context, publicID, currentUserID string) (Post, bool, error) {
	return s.repository.GetPost(ctx, publicID, currentUserID)
}

func (s *Service) DeletePost(ctx context.Context, command DeletePostCommand) error {
	filename, err := s.repository.DeletePost(ctx, command.PublicID, command.UserID)
	if err != nil {
		return err
	}
	if s.files != nil {
		s.files.Delete(filename)
	}
	return nil
}

func (s *Service) LikePost(ctx context.Context, publicID, userID string) error {
	return s.repository.LikePost(ctx, publicID, userID)
}

func (s *Service) UnlikePost(ctx context.Context, publicID, userID string) error {
	return s.repository.UnlikePost(ctx, publicID, userID)
}

func (s *Service) ListPopularPosts(ctx context.Context, viewerID string) ([]Post, error) {
	return s.repository.ListPopularPosts(ctx, viewerID, 20)
}
