package posts

import (
	"context"
	"net/http"
	"strings"
	"time"

	"phasma/backend/internal/httpx"
	"phasma/backend/internal/pagination"
	"phasma/backend/internal/validation"
)

type Post struct {
	ID          int       `json:"-"`
	PublicID    string    `json:"publicId"`
	UserID      int       `json:"-"`
	Username    string    `json:"username"`
	Name        string    `json:"name"`
	Avatar      *string   `json:"avatar"`
	Filename    string    `json:"filename"`
	Description *string   `json:"description"`
	Likes       int       `json:"likes"`
	Liked       bool      `json:"liked"`
	Comments    int       `json:"comments"`
	Created     time.Time `json:"created"`
}

type Application interface {
	CreatePost(ctx context.Context, command CreatePostCommand) (CreatePostResult, error)
	GetPosts(ctx context.Context, query ListQuery) ([]Post, *pagination.Cursor, error)
	GetLikedPosts(ctx context.Context, query ListQuery) ([]Post, *pagination.Cursor, error)
	GetPost(ctx context.Context, publicID, currentUserID string) (Post, bool, error)
	DeletePost(ctx context.Context, command DeletePostCommand) error
	LikePost(ctx context.Context, publicID, userID string) error
	UnlikePost(ctx context.Context, publicID, userID string) error
	ListPopularPosts(ctx context.Context, viewerID string) ([]Post, error)
}

type Handler struct {
	Service Application
}

func (h Handler) CreatePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, _ := httpx.UserID(r)
	var body struct {
		Filename    string `json:"filename"`
		Description string `json:"description"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.Filename == "" {
		httpx.WriteMessage(w, http.StatusBadRequest, "Post filename is required.")
		return
	}
	body.Description = strings.TrimSpace(body.Description)
	if len([]rune(body.Description)) > 1000 {
		httpx.WriteMessage(w, http.StatusBadRequest, "Description must be 1000 characters or fewer.")
		return
	}

	var description *string
	if body.Description != "" {
		description = &body.Description
	}
	result, err := h.Service.CreatePost(ctx, CreatePostCommand{
		UserID: userID, Filename: body.Filename, Description: description,
	})
	if err != nil {
		httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}
	if !result.Created {
		httpx.WriteMessage(w, http.StatusBadRequest, "Upload is invalid or expired.")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"publicId": result.PublicID})
}

func (h Handler) GetPosts(w http.ResponseWriter, r *http.Request) {
	h.getPostList(w, r, h.Service.GetPosts)
}

func (h Handler) GetLikedPosts(w http.ResponseWriter, r *http.Request) {
	h.getPostList(w, r, h.Service.GetLikedPosts)
}

func (h Handler) getPostList(w http.ResponseWriter, r *http.Request, fetch func(context.Context, ListQuery) ([]Post, *pagination.Cursor, error)) {
	ctx := r.Context()
	currentUserID, _ := httpx.UserID(r)
	page, ok := pagination.ParsePagination(r.URL.Query())
	if !ok {
		httpx.WriteMessage(w, http.StatusBadRequest, "Invalid pagination parameters.")
		return
	}

	items, nextCursor, err := fetch(ctx, ListQuery{
		Username: strings.ToLower(r.PathValue("username")), Cursor: page.Cursor,
		Limit: page.Limit, CurrentUserID: currentUserID,
	})
	if err != nil {
		httpx.WriteStoreError(w, err)
		return
	}

	writePostPage(w, items, nextCursor)
}

func writePostPage(w http.ResponseWriter, items []Post, nextCursor *pagination.Cursor) {
	httpx.WriteJSON(w, http.StatusOK, pagination.NewCursorPage(items, nextCursor))
}

func (h Handler) GetPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	currentUserID, _ := httpx.UserID(r)
	publicID := r.PathValue("publicId")
	if !validation.ValidUUID(publicID) {
		httpx.WriteMessage(w, http.StatusBadRequest, "Invalid post ID.")
		return
	}
	post, found, err := h.Service.GetPost(ctx, publicID, currentUserID)
	if err != nil {
		httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}
	if !found {
		httpx.WriteMessage(w, http.StatusNotFound, "Not Found")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, post)
}

func (h Handler) DeletePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, _ := httpx.UserID(r)
	publicID := r.PathValue("publicId")
	if !validation.ValidUUID(publicID) {
		httpx.WriteMessage(w, http.StatusBadRequest, "Invalid post ID.")
		return
	}
	err := h.Service.DeletePost(ctx, DeletePostCommand{PublicID: publicID, UserID: userID})
	if err != nil {
		httpx.WriteStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) ListPopularPosts(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := httpx.UserID(r)
	items, err := h.Service.ListPopularPosts(r.Context(), viewerID)
	if err != nil {
		httpx.WriteMessage(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handler) LikePost(w http.ResponseWriter, r *http.Request) {
	h.updateLike(w, r, h.Service.LikePost)
}

func (h Handler) UnlikePost(w http.ResponseWriter, r *http.Request) {
	h.updateLike(w, r, h.Service.UnlikePost)
}

func (h Handler) updateLike(w http.ResponseWriter, r *http.Request, update func(context.Context, string, string) error) {
	ctx := r.Context()
	userID, _ := httpx.UserID(r)
	publicID := r.PathValue("publicId")
	if !validation.ValidUUID(publicID) {
		httpx.WriteMessage(w, http.StatusBadRequest, "Invalid post ID.")
		return
	}
	err := update(ctx, publicID, userID)
	if err != nil {
		httpx.WriteStoreError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
