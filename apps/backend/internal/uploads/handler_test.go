package uploads

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"phasma/backend/internal/blobstore"
	"phasma/backend/internal/httpx"
)

func testJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, 1, 1))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("testJPEG: %v", err)
	}
	return buf.Bytes()
}

type fakeStore struct {
	expired     []string
	createErr   error
	capacity    bool
	err         error
	created     bool
	createdUser string
	filename    string
}

func (s *fakeStore) CreateUpload(_ context.Context, userID, filename string) (bool, []string, error) {
	if s.createErr != nil {
		return false, s.expired, s.createErr
	}
	if s.err != nil {
		return false, s.expired, s.err
	}
	if !s.capacity {
		return false, s.expired, nil
	}
	s.created = true
	s.createdUser = userID
	s.filename = filename
	return true, s.expired, nil
}

func TestCreateFileMissingFile(t *testing.T) {
	store := blobstore.NewMemoryStore()
	handler := Handler{Service: NewService(&fakeStore{capacity: true}), Store: store}
	req, err := multipartRequest(nil)
	if err != nil {
		t.Fatalf("multipartRequest returned error: %v", err)
	}
	req = httpx.WithUserID(req, "1")
	res := httptest.NewRecorder()

	handler.CreateFile(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
	if !strings.Contains(res.Body.String(), "File missing from request.") {
		t.Fatalf("body = %q", res.Body.String())
	}
}

func TestCreateFileRejectsNonImage(t *testing.T) {
	db := &fakeStore{capacity: true}
	store := blobstore.NewMemoryStore()
	handler := Handler{Service: NewService(db), Store: store}
	req, err := multipartRequest([]byte("not an image"))
	if err != nil {
		t.Fatalf("multipartRequest returned error: %v", err)
	}
	req = httpx.WithUserID(req, "1")
	res := httptest.NewRecorder()

	handler.CreateFile(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
	if db.created {
		t.Fatal("non-image upload should not create DB upload")
	}
	if !strings.Contains(res.Body.String(), "Only image uploads are allowed.") {
		t.Fatalf("body = %q", res.Body.String())
	}
}

func TestCreateFileAcceptsImageSignature(t *testing.T) {
	db := &fakeStore{capacity: true}
	store := blobstore.NewMemoryStore()
	handler := Handler{Service: NewService(db), Store: store}
	req, err := multipartRequest(testJPEG(t))
	if err != nil {
		t.Fatalf("multipartRequest returned error: %v", err)
	}
	req = httpx.WithUserID(req, "1")
	res := httptest.NewRecorder()

	handler.CreateFile(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusCreated, res.Body.String())
	}
	if !db.created || db.createdUser != "1" || db.filename == "" {
		t.Fatalf("store create = %v user=%q filename=%q", db.created, db.createdUser, db.filename)
	}
	rc, _, _, err := store.Get(context.Background(), db.filename)
	if err != nil {
		t.Fatalf("object not found in blobstore after upload: %v", err)
	}
	rc.Close()
}

func TestCreateFileDeletesExpiredUploads(t *testing.T) {
	expired := "aabbccddeeff00112233445566778899"
	db := &fakeStore{capacity: true, expired: []string{expired}}
	store := blobstore.NewMemoryStore()
	_ = store.Put(context.Background(), expired, "image/jpeg", bytes.NewReader([]byte{0xff}), 1)
	handler := Handler{Service: NewService(db), Store: store}
	req, err := multipartRequest(testJPEG(t))
	if err != nil {
		t.Fatalf("multipartRequest returned error: %v", err)
	}
	req = httpx.WithUserID(req, "1")
	res := httptest.NewRecorder()

	handler.CreateFile(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusCreated)
	}
	_, _, _, err = store.Get(context.Background(), expired)
	if !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatal("expected expired upload to be deleted from blobstore")
	}
}

func TestCreateFileDeletesExpiredUploadsWhenCreateFails(t *testing.T) {
	expired := "aabbccddeeff00112233445566778899"
	db := &fakeStore{expired: []string{expired}, createErr: errors.New("database unavailable")}
	store := blobstore.NewMemoryStore()
	_ = store.Put(context.Background(), expired, "image/jpeg", bytes.NewReader([]byte{0xff}), 1)
	handler := Handler{Service: NewService(db), Store: store}
	req, err := multipartRequest(testJPEG(t))
	if err != nil {
		t.Fatalf("multipartRequest returned error: %v", err)
	}
	req = httpx.WithUserID(req, "1")
	res := httptest.NewRecorder()

	handler.CreateFile(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
	_, _, _, err = store.Get(context.Background(), expired)
	if !errors.Is(err, blobstore.ErrNotFound) {
		t.Fatal("expected expired upload to be deleted from blobstore even on create failure")
	}
}

func TestCreateFileRejectsQuotaExhaustion(t *testing.T) {
	store := blobstore.NewMemoryStore()
	handler := Handler{Service: NewService(&fakeStore{capacity: false}), Store: store}
	req, err := multipartRequest(testJPEG(t))
	if err != nil {
		t.Fatalf("multipartRequest returned error: %v", err)
	}
	req = httpx.WithUserID(req, "1")
	res := httptest.NewRecorder()

	handler.CreateFile(res, req)

	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusTooManyRequests)
	}
}

func TestCreateFileRejectsOversizedUpload(t *testing.T) {
	store := blobstore.NewMemoryStore()
	handler := Handler{Service: NewService(&fakeStore{capacity: true}), Store: store}
	req, err := multipartRequest(append([]byte{0xff, 0xd8, 0xff}, bytes.Repeat([]byte("x"), fileLimit)...))
	if err != nil {
		t.Fatalf("multipartRequest returned error: %v", err)
	}
	req = httpx.WithUserID(req, "1")
	res := httptest.NewRecorder()

	handler.CreateFile(res, req)

	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestServeFile(t *testing.T) {
	store := blobstore.NewMemoryStore()
	filename := "0123456789abcdef0123456789abcdef"
	content := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00}
	_ = store.Put(context.Background(), filename, "image/png", bytes.NewReader(content), int64(len(content)))

	req := httptest.NewRequest(http.MethodGet, "/uploads/"+filename, nil)
	res := httptest.NewRecorder()

	Handler{Store: store}.ServeFile(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if !bytes.Equal(res.Body.Bytes(), content) {
		t.Fatalf("body = %v, want %v", res.Body.Bytes(), content)
	}
	if got := res.Header().Get("ETag"); got != `"`+filename+`"` {
		t.Errorf("ETag = %q", got)
	}
	if got := res.Header().Get("Cache-Control"); got != "public, max-age=86400, immutable" {
		t.Errorf("Cache-Control = %q", got)
	}
	if got := res.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q", got)
	}
}

func TestServeFileMissingKey(t *testing.T) {
	store := blobstore.NewMemoryStore()
	filename := "0123456789abcdef0123456789abcdef"

	req := httptest.NewRequest(http.MethodGet, "/uploads/"+filename, nil)
	res := httptest.NewRecorder()

	Handler{Store: store}.ServeFile(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
}

func TestServeFileRejectsInvalidOrMissingFilenames(t *testing.T) {
	store := blobstore.NewMemoryStore()
	filename := "0123456789abcdef0123456789abcdef"

	for _, path := range []string{
		"/uploads/",
		"/uploads/not-a-valid-filename",
		"/uploads/0123456789ABCDEF0123456789ABCDEF",
		"/uploads/" + filename + "/extra",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			res := httptest.NewRecorder()

			Handler{Store: store}.ServeFile(res, req)

			if res.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
			}
		})
	}
}

func multipartRequest(file []byte) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if file != nil {
		part, err := writer.CreateFormFile("image", "test.jpg")
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(file); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req := httptest.NewRequest(http.MethodPost, "/uploads", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}
