package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestUploadImage_ValidJPEG(t *testing.T) {
	dir := t.TempDir()
	h := NewUploadHandler(dir)

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, _ := w.CreateFormFile("file", "test.jpg")
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	_ = jpeg.Encode(part, img, nil)
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload/image", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req = req.WithContext(userCtx(req.Context()))

	rr := httptest.NewRecorder()
	h.UploadImage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["url"] == "" {
		t.Fatal("expected url in response")
	}
}

func TestUploadImage_ValidPNG(t *testing.T) {
	dir := t.TempDir()
	h := NewUploadHandler(dir)

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, _ := w.CreateFormFile("file", "test.png")
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	_ = png.Encode(part, img)
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload/image", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req = req.WithContext(userCtx(req.Context()))

	rr := httptest.NewRecorder()
	h.UploadImage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestUploadImage_InvalidMIME(t *testing.T) {
	dir := t.TempDir()
	h := NewUploadHandler(dir)

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, _ := w.CreateFormFile("file", "test.txt")
	_, _ = part.Write([]byte("this is not an image"))
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload/image", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req = req.WithContext(userCtx(req.Context()))

	rr := httptest.NewRecorder()
	h.UploadImage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUploadImage_MissingFile(t *testing.T) {
	dir := t.TempDir()
	h := NewUploadHandler(dir)

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload/image", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req = req.WithContext(userCtx(req.Context()))

	rr := httptest.NewRecorder()
	h.UploadImage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestServeFile(t *testing.T) {
	dir := t.TempDir()
	h := NewUploadHandler(dir)

	_ = os.MkdirAll(dir+"/1/images", 0o755)
	_ = os.WriteFile(dir+"/1/images/test.jpg", []byte("fake-image-data"), 0o644)

	req := httptest.NewRequest(http.MethodGet, "/api/uploads/1/images/test.jpg", nil)
	req = req.WithContext(userCtx(req.Context()))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", "1/images/test.jpg")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.ServeFile(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestServeFile_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	h := NewUploadHandler(dir)

	req := httptest.NewRequest(http.MethodGet, "/api/uploads/../etc/passwd", nil)
	req = req.WithContext(userCtx(req.Context()))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", "../etc/passwd")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.ServeFile(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestServeFile_EmptyPath(t *testing.T) {
	dir := t.TempDir()
	h := NewUploadHandler(dir)

	req := httptest.NewRequest(http.MethodGet, "/api/uploads/", nil)
	req = req.WithContext(userCtx(req.Context()))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.ServeFile(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestServeFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	h := NewUploadHandler(dir)

	req := httptest.NewRequest(http.MethodGet, "/api/uploads/1/images/missing.jpg", nil)
	req = req.WithContext(userCtx(req.Context()))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", "1/images/missing.jpg")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.ServeFile(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestServeFile_OtherUserPath(t *testing.T) {
	dir := t.TempDir()
	h := NewUploadHandler(dir)

	// user 1 in context, trying to access user 2's file
	req := httptest.NewRequest(http.MethodGet, "/api/uploads/2/images/test.jpg", nil)
	req = req.WithContext(userCtx(req.Context()))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", "2/images/test.jpg")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.ServeFile(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}
