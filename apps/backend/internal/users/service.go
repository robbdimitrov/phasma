package users

import (
	"context"
	"log/slog"
	"strconv"

	"phasma/backend/internal/auth"
	"phasma/backend/internal/blobstore"
	"phasma/backend/internal/pagination"
	"phasma/backend/internal/store"
)

type Service struct {
	repository Repository
	blobs      blobstore.Store
}

var _ HandlerService = (*Service)(nil)

func NewService(repository Repository, blobs blobstore.Store) *Service {
	return &Service{repository: repository, blobs: blobs}
}

func (s *Service) CreateUser(ctx context.Context, command CreateUserCommand) (int, error) {
	passwordHash, err := auth.HashPassword(ctx, command.Password, auth.DefaultPasswordParams)
	if err != nil {
		return 0, err
	}

	return s.repository.CreateUser(
		ctx,
		command.Name,
		command.Username,
		command.Email,
		passwordHash,
	)
}

func (s *Service) GetUserByUsername(ctx context.Context, username, currentUserID string) (User, bool, error) {
	return s.repository.GetUserByUsername(ctx, username, currentUserID)
}

func (s *Service) GetUserByID(ctx context.Context, userID, currentUserID string) (User, bool, error) {
	return s.repository.GetUserByID(ctx, userID, currentUserID)
}

func (s *Service) ListFollowers(ctx context.Context, query ListQuery) ([]User, *pagination.Cursor, error) {
	return s.repository.ListFollowers(ctx, query.Username, query.Cursor, query.Limit, query.CurrentUserID)
}

func (s *Service) ListFollowing(ctx context.Context, query ListQuery) ([]User, *pagination.Cursor, error) {
	return s.repository.ListFollowing(ctx, query.Username, query.Cursor, query.Limit, query.CurrentUserID)
}

func (s *Service) UpdateProfile(ctx context.Context, command UpdateProfileCommand) (UpdateProfileOutcome, error) {
	result, err := s.repository.UpdateUser(
		ctx,
		command.UserID,
		command.Name,
		command.Username,
		command.Email,
		command.Avatar,
		command.Bio,
	)
	if err != nil {
		return UpdateProfileUpdated, err
	}
	if !result.Updated {
		return UpdateProfileInvalidAvatar, nil
	}

	if result.UnusedAvatar != "" {
		if err := s.blobs.Delete(ctx, result.UnusedAvatar); err != nil {
			slog.Warn("failed to delete unused avatar blob", "filename", result.UnusedAvatar, "error", err)
		}
	}
	return UpdateProfileUpdated, nil
}

func (s *Service) ChangePassword(ctx context.Context, command ChangePasswordCommand) (ChangePasswordOutcome, error) {
	user, found, err := s.repository.GetUserWithID(ctx, command.UserID)
	if err != nil {
		return ChangePasswordChanged, err
	}
	if !found {
		return ChangePasswordUserNotFound, nil
	}

	valid, err := auth.VerifyPassword(ctx, command.CurrentPassword, user.PasswordHash)
	if err != nil || !valid {
		return ChangePasswordWrongPassword, nil
	}

	passwordHash, err := auth.HashPassword(ctx, command.NewPassword, auth.DefaultPasswordParams)
	if err != nil {
		return ChangePasswordChanged, err
	}
	if err := s.repository.ChangePassword(
		ctx, command.UserID, passwordHash, command.CurrentSessionID,
	); err != nil {
		return ChangePasswordChanged, err
	}

	return ChangePasswordChanged, nil
}

func (s *Service) FollowUser(ctx context.Context, command FollowCommand) error {
	target, found, err := s.repository.GetUserByUsername(ctx, command.FolloweeUsername, command.FollowerID)
	if err != nil {
		return err
	}
	if !found {
		return store.ErrNotFound
	}
	followeeID := strconv.Itoa(target.ID)
	if followeeID == command.FollowerID {
		return ErrSelfFollow
	}
	return s.repository.FollowUser(ctx, command.FollowerID, followeeID)
}

func (s *Service) UnfollowUser(ctx context.Context, command FollowCommand) error {
	target, found, err := s.repository.GetUserByUsername(ctx, command.FolloweeUsername, command.FollowerID)
	if err != nil {
		return err
	}
	if !found {
		return store.ErrNotFound
	}
	return s.repository.UnfollowUser(ctx, command.FollowerID, strconv.Itoa(target.ID))
}

func (s *Service) ListSuggestedUsers(ctx context.Context, viewerID string) ([]User, error) {
	return s.repository.ListSuggestedUsers(ctx, viewerID, 5)
}
