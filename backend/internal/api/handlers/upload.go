package handlers

import (
	"context"
	"fmt"
	"io"
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

type UploadHandler struct {
	uploadDir string
}

func NewUploadHandler(uploadDir string) *UploadHandler {
	return &UploadHandler{uploadDir: uploadDir}
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
func RegisterUploadRoutes(api huma.API, uploadDir string) {
	h := NewUploadHandler(uploadDir)
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

	userDir := filepath.Join(h.uploadDir, strconv.FormatInt(user.UserID, 10), "images")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return nil, huma.Error500InternalServerError("failed to create upload directory")
	}
	filename := uuid.New().String() + ext
	dst, err := os.Create(filepath.Join(userDir, filename))
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

	userPrefix := strconv.FormatInt(user.UserID, 10) + "/"
	if !strings.HasPrefix(relPath, userPrefix) {
		respondError(w, http.StatusForbidden, "access denied")
		return
	}

	clean := filepath.Clean("/" + relPath)
	absPath := filepath.Join(h.uploadDir, clean)
	rel, err := filepath.Rel(h.uploadDir, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		respondError(w, http.StatusBadRequest, "invalid path")
		return
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		respondError(w, http.StatusNotFound, "file not found")
		return
	}

	http.ServeFile(w, r, absPath)
}
