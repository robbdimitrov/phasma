package posts

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"phasma/backend/internal/blobstore"
	"phasma/backend/internal/httpx"
	"phasma/backend/internal/pagination"
	"phasma/backend/internal/store"
	"phasma/backend/internal/uploads"
)

const testPublicID = "550e8400-e29b-41d4-a716-446655440000"

type fakeStore struct {
	post            Post
	posts           []Post
	found           bool
	err             error
	createdID       string
	created         bool
	deleted         bool
	deletedFile     string
	exists          bool
	liked           bool
	unliked         bool
	requestedUser   string
	requestedCursor *pagination.Cursor
	requestedLimit  int
	nextCursor      *pagination.Cursor
}

func (s *fakeStore) CreatePost(_ context.Context, _ string, _ string, _ *string, _ []string) (string, bool, error) {
	return s.createdID, s.created, s.err
}

func (s *fakeStore) GetPosts(_ context.Context, userID string, cursor *pagination.Cursor, limit int, _ string) ([]Post, *pagination.Cursor, error) {
	s.requestedUser = userID
	s.requestedCursor = cursor
	s.requestedLimit = limit
	return s.posts, s.nextCursor, s.err
}

func (s *fakeStore) GetLikedPosts(_ context.Context, _ string, _ *pagination.Cursor, _ int, _ string) ([]Post, *pagination.Cursor, error) {
	return s.posts, s.nextCursor, s.err
}

func (s *fakeStore) GetPost(_ context.Context, _ string, _ string) (Post, bool, error) {
	return s.post, s.found, s.err
}

func (s *fakeStore) DeletePost(_ context.Context, _ string, _ string) (string, error) {
	if s.found && !s.deleted {
		return "", store.ErrForbidden
	}
	if !s.deleted {
		return "", store.ErrNotFound
	}
	return s.deletedFile, s.err
}

func (s *fakeStore) LikePost(_ context.Context, _ string, _ string) error {
	if !s.exists {
		return store.ErrNotFound
	}
	s.liked = true
	return s.err
}

func (s *fakeStore) UnlikePost(_ context.Context, _ string, _ string) error {
	if !s.exists {
		return store.ErrNotFound
	}
	s.unliked = true
	return s.err
}

func (s *fakeStore) ListPopularPosts(_ context.Context, _ string, _ int) ([]Post, error) {
	return s.posts, s.err
}

func TestCreatePostMissingFilename(t *testing.T) {
	handler := Handler{Service: NewService(&fakeStore{}, nil)}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(`{}`))
	req = httpx.WithUserID(req, "1")

	handler.CreatePost(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
	if !strings.Contains(res.Body.String(), "Post filename is required.") {
		t.Fatalf("body = %q", res.Body.String())
	}
}

func TestCreatePostInvalidUpload(t *testing.T) {
	handler := Handler{Service: NewService(&fakeStore{created: false}, nil)}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(`{"filename":"upload"}`))
	req = httpx.WithUserID(req, "1")

	handler.CreatePost(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
	if !strings.Contains(res.Body.String(), "Upload is invalid or expired.") {
		t.Fatalf("body = %q", res.Body.String())
	}
}

func TestCreatePostSuccess(t *testing.T) {
	handler := Handler{Service: NewService(&fakeStore{createdID: testPublicID, created: true}, nil)}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(`{"filename":"upload","description":"hi"}`))
	req = httpx.WithUserID(req, "1")

	handler.CreatePost(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusCreated)
	}
	if strings.TrimSpace(res.Body.String()) != `{"publicId":"`+testPublicID+`"}` {
		t.Fatalf("body = %q", res.Body.String())
	}
}

func TestGetPostsPagination(t *testing.T) {
	cursor := pagination.Cursor{Created: time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC), ID: 42}
	store := &fakeStore{posts: []Post{{ID: 1, Filename: "a"}}}
	handler := Handler{Service: NewService(store, nil)}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/test/posts?cursor="+pagination.EncodeCursor(cursor)+"&limit=25", nil)
	req.SetPathValue("username", "test")
	req = httpx.WithUserID(req, "1")

	handler.GetPosts(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if store.requestedUser != "test" || store.requestedCursor == nil || *store.requestedCursor != cursor || store.requestedLimit != 25 {
		t.Fatalf(
			"request = user %q cursor %+v limit %d",
			store.requestedUser, store.requestedCursor, store.requestedLimit,
		)
	}
}

func TestGetPostNotFound(t *testing.T) {
	handler := Handler{Service: NewService(&fakeStore{}, nil)}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/posts/"+testPublicID, nil)
	req.SetPathValue("publicId", testPublicID)
	req = httpx.WithUserID(req, "1")

	handler.GetPost(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestGetPostRejectsMalformedUUID(t *testing.T) {
	handler := Handler{Service: NewService(&fakeStore{found: true}, nil)}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/posts/not-a-uuid", nil)
	req.SetPathValue("publicId", "not-a-uuid")
	req = httpx.WithUserID(req, "1")

	handler.GetPost(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func TestDeletePostDeletesFile(t *testing.T) {
	store := blobstore.NewMemoryStore()
	filename := "aabbccddeeff00112233445566778899"
	_ = store.Put(context.Background(), filename, "image/jpeg", bytes.NewReader([]byte{0xff}), 1)
	handler := Handler{Service: NewService(
		&fakeStore{deleted: true, deletedFile: filename},
		uploads.Files{Store: store},
	)}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/posts/"+testPublicID, nil)
	req.SetPathValue("publicId", testPublicID)
	req = httpx.WithUserID(req, "1")

	handler.DeletePost(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
	_, _, _, err := store.Get(context.Background(), filename)
	if !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatal("expected file to be deleted from blobstore")
	}
}

func TestDeletePostForbidden(t *testing.T) {
	handler := Handler{Service: NewService(&fakeStore{found: true}, nil)}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/posts/"+testPublicID, nil)
	req.SetPathValue("publicId", testPublicID)
	req = httpx.WithUserID(req, "2")

	handler.DeletePost(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
	}
}

func TestLikePostNotFound(t *testing.T) {
	handler := Handler{Service: NewService(&fakeStore{exists: false}, nil)}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/posts/"+testPublicID+"/likes", nil)
	req.SetPathValue("publicId", testPublicID)
	req = httpx.WithUserID(req, "2")

	handler.LikePost(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestUnlikePostSuccess(t *testing.T) {
	store := &fakeStore{exists: true}
	handler := Handler{Service: NewService(store, nil)}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/posts/"+testPublicID+"/likes", nil)
	req.SetPathValue("publicId", testPublicID)
	req = httpx.WithUserID(req, "2")

	handler.UnlikePost(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
	if !store.unliked {
		t.Fatal("expected UnlikePost store call")
	}
}

func TestLikePostSuccess(t *testing.T) {
	store := &fakeStore{exists: true}
	handler := Handler{Service: NewService(store, nil)}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/posts/"+testPublicID+"/likes", nil)
	req.SetPathValue("publicId", testPublicID)
	req = httpx.WithUserID(req, "2")

	handler.LikePost(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
	if !store.liked {
		t.Fatal("expected LikePost store call")
	}
}

func TestListPopularPostsReturnsItems(t *testing.T) {
	store := &fakeStore{posts: []Post{{ID: 1, Filename: "a"}, {ID: 2, Filename: "b"}}}
	handler := Handler{Service: NewService(store, nil)}
	res := httptest.NewRecorder()
	req := httpx.WithUserID(httptest.NewRequest(http.MethodGet, "/posts/popular", nil), "3")

	handler.ListPopularPosts(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if !strings.Contains(res.Body.String(), `"items"`) {
		t.Fatalf("body = %q, want items key", res.Body.String())
	}
}
