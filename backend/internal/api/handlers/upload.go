package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

func (h *UploadHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())

	if err := r.ParseMultipartForm(5 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "file too large (max 5MB)")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer func() { _ = file.Close() }()

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil {
		respondError(w, http.StatusBadRequest, "cannot read file")
		return
	}
	mime := http.DetectContentType(buf[:n])
	ext, ok := allowedMIME[mime]
	if !ok {
		respondError(w, http.StatusBadRequest, "unsupported file type; allowed: jpeg, png, webp")
		return
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to process file")
		return
	}

	userDir := filepath.Join(h.uploadDir, strconv.FormatInt(user.UserID, 10), "images")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create upload directory")
		return
	}

	filename := uuid.New().String() + ext
	dst, err := os.Create(filepath.Join(userDir, filename))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save file")
		return
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, file); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to write file")
		return
	}

	url := fmt.Sprintf("/api/uploads/%d/images/%s", user.UserID, filename)
	respondJSON(w, http.StatusOK, map[string]string{"url": url})
}

func (h *UploadHandler) ServeFile(w http.ResponseWriter, r *http.Request) {
	relPath := chi.URLParam(r, "*")
	if relPath == "" || strings.Contains(relPath, "..") {
		respondError(w, http.StatusBadRequest, "invalid path")
		return
	}

	absPath := filepath.Join(h.uploadDir, relPath)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		respondError(w, http.StatusNotFound, "file not found")
		return
	}

	http.ServeFile(w, r, absPath)
}
