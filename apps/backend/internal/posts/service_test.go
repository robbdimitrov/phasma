package posts

import (
	"context"
	"errors"
	"testing"

	"phasma/backend/internal/store"
)

type serviceRepository struct {
	fakeStore
	deletedFile string
	deleted     bool
	found       bool
}

func (r *serviceRepository) DeletePost(context.Context, string, string) (string, error) {
	if r.found && !r.deleted {
		return "", store.ErrForbidden
	}
	if !r.deleted {
		return "", store.ErrNotFound
	}
	return r.deletedFile, nil
}
func (r *serviceRepository) GetPost(context.Context, string, string) (Post, bool, error) {
	return Post{}, r.found, nil
}
func (r *serviceRepository) ListPopularPosts(context.Context, string, int) ([]Post, error) {
	return nil, nil
}

type fakeFiles struct{ deleted string }

func (f *fakeFiles) Delete(filename string) { f.deleted = filename }

func TestServiceDeletePostRemovesFile(t *testing.T) {
	repository := &serviceRepository{deleted: true, deletedFile: "post-file"}
	files := &fakeFiles{}

	err := NewService(repository, files).DeletePost(
		context.Background(), DeletePostCommand{PublicID: testPublicID, UserID: "1"},
	)

	if err != nil || files.deleted != "post-file" {
		t.Fatalf("DeletePost() error = %v, deleted file = %q", err, files.deleted)
	}
}

func TestServiceDeletePostDistinguishesForbidden(t *testing.T) {
	repository := &serviceRepository{found: true}

	err := NewService(repository, nil).DeletePost(
		context.Background(), DeletePostCommand{PublicID: testPublicID, UserID: "2"},
	)

	if !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("DeletePost() error = %v, want forbidden", err)
	}
}

func TestServiceLikePostRequiresExistingPost(t *testing.T) {
	err := NewService(&serviceRepository{}, nil).LikePost(
		context.Background(), testPublicID, "1",
	)

	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("LikePost() error = %v, want not found", err)
	}
}
