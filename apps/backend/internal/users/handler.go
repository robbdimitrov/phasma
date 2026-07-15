package users

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"phasma/backend/internal/httpx"
	"phasma/backend/internal/pagination"
	"phasma/backend/internal/store"
	"phasma/backend/internal/validation"
)

type HandlerService interface {
	CreateUser(ctx context.Context, command CreateUserCommand) (int, error)
	GetUserByUsername(ctx context.Context, username, currentUserID string) (User, bool, error)
	GetUserByID(ctx context.Context, userID, currentUserID string) (User, bool, error)
	ListFollowers(ctx context.Context, query ListQuery) ([]User, *pagination.Cursor, error)
	ListFollowing(ctx context.Context, query ListQuery) ([]User, *pagination.Cursor, error)
	UpdateProfile(ctx context.Context, command UpdateProfileCommand) (UpdateProfileOutcome, error)
	ChangePassword(ctx context.Context, command ChangePasswordCommand) (ChangePasswordOutcome, error)
	FollowUser(ctx context.Context, command FollowCommand) error
	UnfollowUser(ctx context.Context, command FollowCommand) error
	ListSuggestedUsers(ctx context.Context, viewerID string) ([]User, error)
}

type Handler struct {
	Service HandlerService
}

func NewHandler(service HandlerService) Handler {
	return Handler{Service: service}
}

func (h Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}

	name := strings.TrimSpace(body.Name)
	username := strings.ToLower(strings.TrimSpace(body.Username))
	email := strings.ToLower(strings.TrimSpace(body.Email))

	if name == "" || username == "" || email == "" || body.Password == "" {
		httpx.WriteMessage(w, http.StatusBadRequest, "Name, username, email and password are required.")
		return
	}
	if len([]rune(name)) > 100 {
		httpx.WriteMessage(w, http.StatusBadRequest, "Name must be 100 characters or fewer.")
		return
	}
	if !validation.ValidUsername(username) {
		httpx.WriteMessage(w, http.StatusBadRequest, "Username must be 3-30 characters and contain only lowercase letters, numbers, periods, or underscores.")
		return
	}
	if !validation.ValidPassword(body.Password) {
		httpx.WriteMessage(w, http.StatusBadRequest, "Password must be between 8 and 128 characters long.")
		return
	}
	if !validation.ValidEmail(email) {
		httpx.WriteMessage(w, http.StatusBadRequest, "Invalid email address.")
		return
	}

	_, err := h.Service.CreateUser(r.Context(), CreateUserCommand{
		Name: name, Username: username, Email: email, Password: body.Password,
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			httpx.WriteMessage(w, http.StatusConflict, "User with this username or email already exists.")
			return
		}
		httpx.WriteMessage(w, http.StatusInternalServerError, "Could not create user. Please try again.")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"username": username})
}

func (h Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	username := strings.ToLower(r.PathValue("username"))
	if !validation.ValidUsername(username) {
		httpx.WriteMessage(w, http.StatusBadRequest, "Invalid username.")
		return
	}
	currentUserID, currentUserOK := httpx.UserID(r)

	user, found, err := h.Service.GetUserByUsername(r.Context(), username, currentUserID)
	if err != nil {
		httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}
	if !found {
		httpx.WriteMessage(w, http.StatusNotFound, "Not Found")
		return
	}

	if !currentUserOK || currentUserID != strconv.Itoa(user.ID) {
		user.Email = ""
	}

	httpx.WriteJSON(w, http.StatusOK, user)
}

func (h Handler) ListFollowers(w http.ResponseWriter, r *http.Request) {
	h.listUsers(w, r, h.Service.ListFollowers)
}

func (h Handler) ListFollowing(w http.ResponseWriter, r *http.Request) {
	h.listUsers(w, r, h.Service.ListFollowing)
}

func (h Handler) listUsers(
	w http.ResponseWriter,
	r *http.Request,
	fetch func(context.Context, ListQuery) ([]User, *pagination.Cursor, error),
) {
	currentUserID, _ := httpx.UserID(r)
	page, ok := pagination.ParsePagination(r.URL.Query())
	if !ok {
		httpx.WriteMessage(w, http.StatusBadRequest, "Invalid pagination parameters.")
		return
	}

	items, nextCursor, err := fetch(r.Context(), ListQuery{
		Username: strings.ToLower(r.PathValue("username")), CurrentUserID: currentUserID,
		Cursor: page.Cursor, Limit: page.Limit,
	})
	if err != nil {
		httpx.WriteStoreError(w, err)
		return
	}
	for i := range items {
		if strconv.Itoa(items[i].ID) != currentUserID {
			items[i].Email = ""
		}
	}

	httpx.WriteJSON(w, http.StatusOK, pagination.NewCursorPage(items, nextCursor))
}

func (h Handler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	currentUserID, ok := httpx.UserID(r)
	if !ok {
		httpx.WriteMessage(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, found, err := h.Service.GetUserByID(r.Context(), currentUserID, currentUserID)
	if err != nil {
		httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}
	if !found {
		httpx.WriteMessage(w, http.StatusNotFound, "Not Found")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, user)
}

func (h Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	currentUserID, ok := httpx.UserID(r)
	if !ok {
		httpx.WriteMessage(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userID := currentUserID

	var body struct {
		Name        string  `json:"name"`
		Username    string  `json:"username"`
		Email       string  `json:"email"`
		Avatar      *string `json:"avatar"`
		Bio         *string `json:"bio"`
		Password    string  `json:"password"`
		OldPassword string  `json:"oldPassword"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}

	if body.Password != "" {
		h.updatePassword(w, r, userID, body.OldPassword, body.Password)
		return
	}

	name := strings.TrimSpace(body.Name)
	username := strings.ToLower(strings.TrimSpace(body.Username))
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if name == "" || username == "" || email == "" {
		httpx.WriteMessage(w, http.StatusBadRequest, "Name, username, and email are required.")
		return
	}
	if len([]rune(name)) > 100 {
		httpx.WriteMessage(w, http.StatusBadRequest, "Name must be 100 characters or fewer.")
		return
	}
	if !validation.ValidUsername(username) {
		httpx.WriteMessage(w, http.StatusBadRequest, "Username must be 3-30 characters and contain only lowercase letters, numbers, periods, or underscores.")
		return
	}
	if !validation.ValidEmail(email) {
		httpx.WriteMessage(w, http.StatusBadRequest, "Invalid email address.")
		return
	}

	avatar := ""
	if body.Avatar != nil {
		avatar = *body.Avatar
	}
	if body.Bio != nil {
		trimmedBio := strings.TrimSpace(*body.Bio)
		body.Bio = &trimmedBio
	}
	if body.Bio != nil && len([]rune(*body.Bio)) > 300 {
		httpx.WriteMessage(w, http.StatusBadRequest, "Bio must be 300 characters or fewer.")
		return
	}

	outcome, err := h.Service.UpdateProfile(r.Context(), UpdateProfileCommand{
		UserID: userID, Name: name, Username: username, Email: email, Avatar: avatar, Bio: body.Bio,
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			httpx.WriteMessage(w, http.StatusConflict, "This username or email is already in use.")
			return
		}
		httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}
	if outcome == UpdateProfileInvalidAvatar {
		httpx.WriteMessage(w, http.StatusBadRequest, "Avatar upload is invalid or expired.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) updatePassword(w http.ResponseWriter, r *http.Request, userID, oldPassword, password string) {
	if oldPassword == "" || password == "" {
		httpx.WriteMessage(w, http.StatusBadRequest, "Both password and the current password are required.")
		return
	}
	if !validation.ValidPassword(password) {
		httpx.WriteMessage(w, http.StatusBadRequest, "Password must be between 8 and 128 characters long.")
		return
	}

	currentSessionID, _ := httpx.GetSessionCookie(r)
	outcome, err := h.Service.ChangePassword(r.Context(), ChangePasswordCommand{
		UserID: userID, CurrentPassword: oldPassword, NewPassword: password, CurrentSessionID: currentSessionID,
	})
	if err != nil {
		httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}
	switch outcome {
	case ChangePasswordUserNotFound:
		httpx.WriteMessage(w, http.StatusNotFound, "Not Found")
		return
	case ChangePasswordWrongPassword:
		httpx.WriteMessage(w, http.StatusBadRequest, "Wrong password. Enter the correct current password.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) ListSuggestedUsers(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := httpx.UserID(r)
	if !ok {
		httpx.WriteMessage(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	items, err := h.Service.ListSuggestedUsers(r.Context(), viewerID)
	if err != nil {
		httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}
	for i := range items {
		items[i].Email = ""
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handler) FollowUser(w http.ResponseWriter, r *http.Request) {
	h.updateFollow(w, r, h.Service.FollowUser)
}

func (h Handler) UnfollowUser(w http.ResponseWriter, r *http.Request) {
	h.updateFollow(w, r, h.Service.UnfollowUser)
}

func (h Handler) updateFollow(
	w http.ResponseWriter,
	r *http.Request,
	update func(context.Context, FollowCommand) error,
) {
	currentUserID, ok := httpx.UserID(r)
	if !ok {
		httpx.WriteMessage(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	targetUsername := strings.ToLower(r.PathValue("username"))
	if !validation.ValidUsername(targetUsername) {
		httpx.WriteMessage(w, http.StatusBadRequest, "Invalid username.")
		return
	}

	err := update(r.Context(), FollowCommand{FollowerID: currentUserID, FolloweeUsername: targetUsername})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteMessage(w, http.StatusNotFound, "User Not Found")
			return
		}
		if errors.Is(err, ErrSelfFollow) {
			httpx.WriteMessage(w, http.StatusBadRequest, "Cannot follow yourself.")
			return
		}
		httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
