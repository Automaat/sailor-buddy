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
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/go-chi/chi/v5"
)

// testUploadRoot opens dir as the upload root and closes it after the test.
func testUploadRoot(t *testing.T, dir string) *os.Root {
	t.Helper()
	root, err := OpenUploadRoot(dir)
	if err != nil {
		t.Fatalf("open upload root: %v", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root
}

func uploadTestAPI(t *testing.T) (api humatest.TestAPI, dir string) {
	t.Helper()
	dir = t.TempDir()
	_, api = humatest.New(t)
	RegisterUploadRoutes(api, testUploadRoot(t, dir))
	return api, dir
}

// writeUpload creates an uploaded file under dir with the given contents.
func writeUpload(t *testing.T, dir, rel, contents string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
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
	api, dir := uploadTestAPI(t)
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
	// The URL names the stored file under the user's image directory.
	rel, ok := strings.CutPrefix(out["url"], "/api/uploads/")
	if !ok {
		t.Fatalf("url = %q, want /api/uploads/ prefix", out["url"])
	}
	if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
		t.Fatalf("stored file: %v", err)
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
	writeUpload(t, dir, "1/images/test.jpg", "fake-image-data")
	h := NewUploadHandler(testUploadRoot(t, dir))

	rr := serveFileReq(t, h, "1/images/test.jpg")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "fake-image-data" {
		t.Fatalf("body = %q, want the stored file", rr.Body.String())
	}
}

// TestServeFile_Rejected covers request paths that must not serve a file:
// outside the caller's directory, through ".." or a symlink, or not a file.
func TestServeFile_Rejected(t *testing.T) {
	outside := t.TempDir()
	writeUpload(t, outside, "secret.txt", "secret")
	dir := t.TempDir()
	writeUpload(t, dir, "2/images/test.jpg", "other-user")
	writeUpload(t, dir, "1/images/own.jpg", "own")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(dir, "1", "images", "link.jpg")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "1", "out")); err != nil {
		t.Fatal(err)
	}
	h := NewUploadHandler(testUploadRoot(t, dir))

	tests := []struct {
		name     string
		wildcard string
		want     int
	}{
		{"empty path", "", http.StatusBadRequest},
		{"parent traversal", "../etc/passwd", http.StatusForbidden},
		{"traversal after own prefix", "1/../../etc/passwd", http.StatusForbidden},
		{"other user via own prefix", "1/../2/images/test.jpg", http.StatusForbidden},
		{"other user", "2/images/test.jpg", http.StatusForbidden},
		{"missing file", "1/images/missing.jpg", http.StatusNotFound},
		{"directory", "1/images", http.StatusNotFound},
		{"symlinked file outside uploads", "1/images/link.jpg", http.StatusBadRequest},
		{"symlinked directory outside uploads", "1/out/secret.txt", http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr := serveFileReq(t, h, tc.wildcard)
			if rr.Code != tc.want {
				t.Fatalf("got %d, want %d: %s", rr.Code, tc.want, rr.Body.String())
			}
			if strings.Contains(rr.Body.String(), "secret") || strings.Contains(rr.Body.String(), "other-user") {
				t.Fatalf("body leaked file contents: %q", rr.Body.String())
			}
		})
	}
}

func TestOpenUploadRoot_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "uploads")
	root := testUploadRoot(t, dir)
	if root.Name() != dir {
		t.Fatalf("root name = %q, want %q", root.Name(), dir)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("upload dir not created: %v", err)
	}
}

func TestUploadRelPath(t *testing.T) {
	dir := t.TempDir()
	root := testUploadRoot(t, dir)
	tests := []struct {
		name    string
		stored  string
		want    string
		wantErr bool
	}{
		{"inside", filepath.Join(dir, "opinions", "1_2.pdf"), filepath.Join("opinions", "1_2.pdf"), false},
		{"outside", filepath.Join(filepath.Dir(dir), "x.pdf"), "", true},
		{"escapes via dot-dot", filepath.Join(dir, "..", "x.pdf"), "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := uploadRelPath(root, tc.stored)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("uploadRelPath(%q) = %q, %v; want %q, err %v", tc.stored, got, err, tc.want, tc.wantErr)
			}
		})
	}
}
