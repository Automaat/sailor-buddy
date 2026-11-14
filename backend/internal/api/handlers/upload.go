package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
)

// UploadHandler stores and serves uploaded images. Every file access goes
// through uploads, so request paths cannot reach outside the upload
// directory, by ".." or by a symlink.
type UploadHandler struct {
	uploads *os.Root
}

func NewUploadHandler(uploads *os.Root) *UploadHandler {
	return &UploadHandler{uploads: uploads}
}

// OpenUploadRoot creates the upload directory if needed and opens it as the
// root that confines all upload file access. The caller closes it.
func OpenUploadRoot(dir string) (*os.Root, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create upload directory: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open upload directory: %w", err)
	}
	return root, nil
}

// uploadRelPath maps a stored file path, as built by joining the upload
// directory with a relative name, back to its path inside uploads. Both are
// resolved against the working directory, as os.Open would.
func uploadRelPath(uploads *os.Root, stored string) (string, error) {
	base, err := filepath.Abs(uploads.Name())
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(stored)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", err
	}
	if !filepath.IsLocal(rel) {
		return "", fmt.Errorf("path %q is outside the upload directory", stored)
	}
	return rel, nil
}

var allowedMIME = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

type uploadImageInput struct {
	RawBody huma.MultipartFormFiles[struct {
		File huma.FormFile `form:"file" required:"true" doc:"Image file (jpeg, png or webp; max 5 MB)"`
	}]
}

type uploadURLOutput struct {
	Body struct {
		URL string `json:"url"`
	}
}

// RegisterUploadRoutes wires the image upload operation onto the API. Static
// file serving (GET /uploads/*) stays on chi as it is not an API operation.
func RegisterUploadRoutes(api huma.API, uploads *os.Root) {
	h := NewUploadHandler(uploads)
	huma.Register(api, huma.Operation{
		OperationID: "upload-image", Method: http.MethodPost, Path: "/upload/image",
		Summary: "Upload an image", Tags: []string{"Uploads"},
	}, h.uploadImage)
}

func (h *UploadHandler) uploadImage(ctx context.Context, in *uploadImageInput) (*uploadURLOutput, error) {
	user := middleware.GetUser(ctx)
	file := in.RawBody.Data().File
	if !file.IsSet {
		return nil, huma.Error400BadRequest("missing file field")
	}
	defer func() { _ = file.Close() }()
	if file.Size > 5<<20 {
		return nil, huma.Error400BadRequest("file too large (max 5MB)")
	}

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil {
		return nil, huma.Error400BadRequest("cannot read file")
	}
	ext, ok := allowedMIME[http.DetectContentType(buf[:n])]
	if !ok {
		return nil, huma.Error400BadRequest("unsupported file type; allowed: jpeg, png, webp")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, huma.Error500InternalServerError("failed to process file")
	}

	userDir := filepath.Join(strconv.FormatInt(user.UserID, 10), "images")
	if err := h.uploads.MkdirAll(userDir, 0o755); err != nil {
		return nil, huma.Error500InternalServerError("failed to create upload directory")
	}
	filename := uuid.New().String() + ext
	dst, err := h.uploads.Create(filepath.Join(userDir, filename))
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to save file")
	}
	defer func() { _ = dst.Close() }()
	if _, err := io.Copy(dst, file); err != nil {
		return nil, huma.Error500InternalServerError("failed to write file")
	}

	out := &uploadURLOutput{}
	out.Body.URL = fmt.Sprintf("/api/uploads/%d/images/%s", user.UserID, filename)
	return out, nil
}

// ServeFile serves a previously uploaded file. It stays on chi: static asset
// delivery under a wildcard path is not modelled as an API operation.
func (h *UploadHandler) ServeFile(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())

	relPath := chi.URLParam(r, "*")
	if relPath == "" {
		respondError(w, http.StatusBadRequest, "invalid path")
		return
	}

	// Check ownership on the cleaned path, so "1/../2/..." cannot reach
	// another user's files.
	clean := filepath.Clean(relPath)
	userPrefix := strconv.FormatInt(user.UserID, 10) + string(filepath.Separator)
	if !strings.HasPrefix(clean, userPrefix) {
		respondError(w, http.StatusForbidden, "access denied")
		return
	}

	f, err := h.uploads.Open(clean)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			respondError(w, http.StatusNotFound, "file not found")
			return
		}
		slog.Warn("open uploaded file", "path", clean, "err", err)
		respondError(w, http.StatusBadRequest, "invalid path")
		return
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to read file")
		return
	}
	if info.IsDir() {
		respondError(w, http.StatusNotFound, "file not found")
		return
	}

	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}
