package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/go-chi/chi/v5"
)

func uploadTestAPI(t *testing.T) (api humatest.TestAPI, dir string) {
	t.Helper()
	dir = t.TempDir()
	_, api = humatest.New(t)
	RegisterUploadRoutes(api, dir)
	return api, dir
}

// multipartBody builds a multipart form body with one "file" part written by
// encode, returning the body and its Content-Type header value.
func multipartBody(t *testing.T, filename string, encode func(io.Writer)) (body *bytes.Buffer, contentType string) {
	t.Helper()
	body = &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	encode(part)
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return body, w.FormDataContentType()
}

func TestUploadImage_ValidJPEG(t *testing.T) {
	api, _ := uploadTestAPI(t)
	body, ct := multipartBody(t, "test.jpg", func(wr io.Writer) {
		_ = jpeg.Encode(wr, image.NewRGBA(image.Rect(0, 0, 1, 1)), nil)
	})
	resp := api.PostCtx(userCtx(context.Background()), "/upload/image", "Content-Type: "+ct, body)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body)
	}
	var out map[string]string
	_ = json.Unmarshal(resp.Body.Bytes(), &out)
	if out["url"] == "" {
		t.Fatal("expected url in response")
	}
}

func TestUploadImage_ValidPNG(t *testing.T) {
	api, _ := uploadTestAPI(t)
	body, ct := multipartBody(t, "test.png", func(wr io.Writer) {
		_ = png.Encode(wr, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	})
	resp := api.PostCtx(userCtx(context.Background()), "/upload/image", "Content-Type: "+ct, body)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body)
	}
}

func TestUploadImage_InvalidMIME(t *testing.T) {
	api, _ := uploadTestAPI(t)
	body, ct := multipartBody(t, "test.txt", func(wr io.Writer) {
		_, _ = wr.Write([]byte("this is not an image"))
	})
	resp := api.PostCtx(userCtx(context.Background()), "/upload/image", "Content-Type: "+ct, body)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body)
	}
}

func TestUploadImage_MissingFile(t *testing.T) {
	api, _ := uploadTestAPI(t)
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	_ = w.Close()
	// huma rejects the absent required file part with 422.
	resp := api.PostCtx(userCtx(context.Background()), "/upload/image", "Content-Type: "+w.FormDataContentType(), body)
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", resp.Code, resp.Body)
	}
}

func serveFileReq(t *testing.T, h *UploadHandler, wildcard string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/uploads/"+wildcard, http.NoBody)
	req = req.WithContext(userCtx(req.Context()))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", wildcard)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()
	h.ServeFile(rr, req)
	return rr
}

func TestServeFile(t *testing.T) {
	dir := t.TempDir()
	h := NewUploadHandler(dir)
	_ = os.MkdirAll(dir+"/1/images", 0o755)
	_ = os.WriteFile(dir+"/1/images/test.jpg", []byte("fake-image-data"), 0o644)

	if rr := serveFileReq(t, h, "1/images/test.jpg"); rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestServeFile_PathTraversal(t *testing.T) {
	h := NewUploadHandler(t.TempDir())
	if rr := serveFileReq(t, h, "../etc/passwd"); rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestServeFile_EmptyPath(t *testing.T) {
	h := NewUploadHandler(t.TempDir())
	if rr := serveFileReq(t, h, ""); rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestServeFile_NotFound(t *testing.T) {
	h := NewUploadHandler(t.TempDir())
	if rr := serveFileReq(t, h, "1/images/missing.jpg"); rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestServeFile_OtherUserPath(t *testing.T) {
	h := NewUploadHandler(t.TempDir())
	if rr := serveFileReq(t, h, "2/images/test.jpg"); rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}
