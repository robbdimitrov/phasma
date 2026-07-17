package uploads

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	_ "golang.org/x/image/webp"

	"phasma/backend/internal/blobstore"
	"phasma/backend/internal/httpx"
)

const (
	fileLimit      = 1_000_000
	maxImagePixels = 25_000_000 // 5000×5000; guards against decompression bombs
)

type Application interface {
	Register(ctx context.Context, command RegisterCommand) (RegisterResult, error)
}

type Handler struct {
	Service Application
	Store   blobstore.Store
}

func (h Handler) CreateFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, _ := httpx.UserID(r)

	data, err := h.readMultipartImage(r)
	if err != nil {
		status, message := uploadErrorResponse(err)
		httpx.WriteMessage(w, status, message)
		return
	}

	header := data
	if len(header) > 12 {
		header = header[:12]
	}
	if !isImage(header) {
		httpx.WriteMessage(w, http.StatusBadRequest, "Only image uploads are allowed.")
		return
	}

	data, err = processImage(data)
	if err != nil {
		status, message := uploadErrorResponse(err)
		httpx.WriteMessage(w, status, message)
		return
	}

	filename, err := randomFilename()
	if err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "Could not process upload.")
		return
	}

	result, err := h.Service.Register(ctx, RegisterCommand{UserID: userID, Filename: filename})
	for _, f := range result.ExpiredFilenames {
		if delErr := h.Store.Delete(ctx, f); delErr != nil {
			slog.Warn("failed to delete expired upload blob", "filename", f, "error", delErr)
		}
	}
	if err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "Could not process upload.")
		return
	}
	if !result.Created {
		httpx.WriteMessage(w, http.StatusTooManyRequests, "Too many pending uploads. Create posts with existing uploads or try again later.")
		return
	}

	if err := h.Store.Put(ctx, filename, "image/jpeg", bytes.NewReader(data), int64(len(data))); err != nil {
		httpx.WriteMessage(w, http.StatusBadRequest, "Could not process upload.")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"filename": filename})
}

func (h Handler) readMultipartImage(r *http.Request) ([]byte, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, errProcessUpload
	}

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errProcessUpload
		}
		if part.FormName() == "image" {
			data, err := io.ReadAll(io.LimitReader(part, fileLimit+1))
			if err != nil {
				return nil, errProcessUpload
			}
			if int64(len(data)) > fileLimit {
				return nil, errFileTooLarge
			}
			return data, nil
		}
	}
	return nil, errMissingFile
}

func (h Handler) ServeFile(w http.ResponseWriter, r *http.Request) {
	filename, ok := uploadFilename(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	rc, ct, size, err := h.Store.Get(r.Context(), filename)
	if errors.Is(err, blobstore.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer rc.Close()

	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	w.Header().Set("ETag", `"`+filename+`"`)
	if ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	_, _ = io.Copy(w, rc)
}

func uploadFilename(path string) (string, bool) {
	filename, ok := strings.CutPrefix(path, "/uploads/")
	if !ok || len(filename) != 32 {
		return "", false
	}
	for _, char := range filename {
		if !('0' <= char && char <= '9') && !('a' <= char && char <= 'f') {
			return "", false
		}
	}
	return filename, true
}

type Files struct {
	Store blobstore.Store
}

func (f Files) Delete(filename string) {
	if err := f.Store.Delete(context.Background(), filename); err != nil {
		slog.Warn("failed to delete blob", "filename", filename, "error", err)
	}
}

// processImage bounds dimensions and re-encodes to JPEG, stripping metadata
// and rejecting polyglot or crafted payloads.
func processImage(data []byte) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errProcessUpload
	}
	if cfg.Width*cfg.Height > maxImagePixels {
		return nil, errImageTooLarge
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errProcessUpload
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, errProcessUpload
	}
	if buf.Len() > fileLimit {
		return nil, errFileTooLarge
	}
	return buf.Bytes(), nil
}

func uploadErrorResponse(err error) (int, string) {
	switch {
	case errors.Is(err, errMissingFile):
		return http.StatusBadRequest, "File missing from request."
	case errors.Is(err, errFileTooLarge):
		return http.StatusRequestEntityTooLarge, "File too large. Try a smaller image."
	case errors.Is(err, errImageTooLarge):
		return http.StatusBadRequest, "Image dimensions too large."
	default:
		return http.StatusBadRequest, "Could not process upload."
	}
}

var (
	errMissingFile   = errors.New("file missing")
	errFileTooLarge  = errors.New("file too large")
	errImageTooLarge = errors.New("image dimensions too large")
	errProcessUpload = errors.New("could not process upload")
)

type signature struct {
	bytes     []byte
	offset    int
	secondary *signature
}

var signatures = []signature{
	{bytes: []byte{0xff, 0xd8, 0xff}},
	{bytes: []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}},
	{bytes: []byte{0x47, 0x49, 0x46, 0x38}},
	{
		bytes:  []byte{0x52, 0x49, 0x46, 0x46},
		offset: 0,
		secondary: &signature{
			bytes:  []byte{0x57, 0x45, 0x42, 0x50},
			offset: 8,
		},
	},
}

func isImage(buffer []byte) bool {
	for _, candidate := range signatures {
		if !hasBytes(buffer, candidate.bytes, candidate.offset) {
			continue
		}
		if candidate.secondary == nil || hasBytes(buffer, candidate.secondary.bytes, candidate.secondary.offset) {
			return true
		}
	}
	return false
}

func hasBytes(buffer, bytes []byte, offset int) bool {
	if len(buffer) < offset+len(bytes) {
		return false
	}
	for i, b := range bytes {
		if buffer[offset+i] != b {
			return false
		}
	}
	return true
}

func randomFilename() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
