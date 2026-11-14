package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
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

func TestOpenUploadRoot_ChecksWritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	tests := []struct {
		name     string
		readOnly string // path under the upload dir made read-only
		wantErr  bool
	}{
		{name: "writable tree"},
		{name: "read-only top dir", readOnly: ".", wantErr: true},
		{name: "read-only subdir", readOnly: "opinions", wantErr: true},
		{name: "read-only file", readOnly: "opinions/1_2.pdf", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeUpload(t, dir, "opinions/1_2.pdf", "pdf")
			writeUpload(t, dir, "7/images/a.png", "png")
			outside := t.TempDir()
			writeUpload(t, outside, "target.pdf", "target")
			if err := syscall.Mkfifo(filepath.Join(dir, "7", "pipe"), 0o644); err != nil {
				t.Fatal(err)
			}
			links := map[string]string{
				uploadWriteProbe:               filepath.Join(outside, "target.pdf"),
				"opinions/inside.pdf":          filepath.Join(dir, "opinions", "1_2.pdf"),
				"opinions/outside.pdf":         filepath.Join(outside, "target.pdf"),
				"7/images/outside-dir":         outside,
				"7/images/dangling.png":        filepath.Join(outside, "missing.png"),
				"opinions/" + uploadWriteProbe: filepath.Join(dir, "opinions", "1_2.pdf"),
			}
			for link, target := range links {
				if err := os.Symlink(target, filepath.Join(dir, link)); err != nil {
					t.Fatal(err)
				}
			}
			if tt.readOnly != "" {
				target := filepath.Join(dir, tt.readOnly)
				info, err := os.Stat(target)
				if err != nil {
					t.Fatal(err)
				}
				mode := os.FileMode(0o444)
				if info.IsDir() {
					mode = 0o555
				}
				if err := os.Chmod(target, mode); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(target, info.Mode().Perm()) })
			}

			root, err := OpenUploadRoot(dir)
			if root != nil {
				_ = root.Close()
			}
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "not writable") {
					t.Fatalf("want not-writable error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("open writable upload root: %v", err)
			}
			if got, err := os.ReadFile(filepath.Join(dir, "opinions", "1_2.pdf")); err != nil || string(got) != "pdf" {
				t.Fatalf("existing upload changed: %q, %v", got, err)
			}
			if got, err := os.ReadFile(filepath.Join(outside, "target.pdf")); err != nil || string(got) != "target" {
				t.Fatalf("symlink target changed: %q, %v", got, err)
			}
			for link, target := range links {
				if got, err := os.Readlink(filepath.Join(dir, link)); err != nil || got != target {
					t.Errorf("symlink %s = %q, %v; want %q", link, got, err, target)
				}
			}
			assertNoWriteProbes(t, dir)
		})
	}
}

// assertNoWriteProbes fails when a probe file is left in any directory under
// dir.
func assertNoWriteProbes(t *testing.T, dir string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), uploadWriteProbe+"-") {
			t.Errorf("write probe left: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestOpenUploadRoot_Concurrent(t *testing.T) {
	dir := t.TempDir()
	writeUpload(t, dir, "opinions/1_2.pdf", "pdf")
	writeUpload(t, dir, "7/images/a.png", "png")
	const workers, rounds = 8, 25
	errs := make(chan error, workers*rounds)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range rounds {
				root, err := OpenUploadRoot(dir)
				if err != nil {
					errs <- err
					continue
				}
				_ = root.Close()
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent open: %v", err)
	}
	assertNoWriteProbes(t, dir)
}

func TestCheckUploadEntry_DeletedDuringWalk(t *testing.T) {
	dir := t.TempDir()
	writeUpload(t, dir, "gone/a.png", "png")
	root := testUploadRoot(t, dir)
	dirEntry := uploadDirEntry(t, root, ".", "gone")
	fileEntry := uploadDirEntry(t, root, "gone", "a.png")
	if err := os.RemoveAll(filepath.Join(dir, "gone")); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		path    string
		d       fs.DirEntry
		walkErr error
		wantErr error
	}{
		{name: "deleted directory", path: "gone", d: dirEntry},
		{name: "deleted file", path: "gone/a.png", d: fileEntry},
		{name: "unreadable deleted directory", path: "gone", d: dirEntry, walkErr: fs.ErrNotExist},
		{name: "permission error", path: "gone", d: dirEntry, walkErr: fs.ErrPermission, wantErr: fs.ErrPermission},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkUploadEntry(root, uploadWriteProbe+"-test", tt.path, tt.d, tt.walkErr)
			if !errors.Is(err, tt.wantErr) || (tt.wantErr == nil && err != nil) {
				t.Fatalf("checkUploadEntry = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenUploadRoot_DirsDeletedDuringWalk(t *testing.T) {
	dir := t.TempDir()
	writeUpload(t, dir, "opinions/1_2.pdf", "pdf")
	churn := filepath.Join(dir, "churn")
	var stop atomic.Bool
	done := make(chan struct{})
	go func() {
		defer close(done)
		for !stop.Load() {
			_ = os.MkdirAll(filepath.Join(churn, "sub"), 0o755)
			_ = os.WriteFile(filepath.Join(churn, "sub", "a.png"), []byte("png"), 0o644)
			_ = os.RemoveAll(churn)
		}
	}()
	var failures []error
	for range 2000 {
		root, err := OpenUploadRoot(dir)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		_ = root.Close()
	}
	stop.Store(true)
	<-done
	const shown = 5
	for i, err := range failures {
		if i == shown {
			t.Errorf("%d more failures", len(failures)-shown)
			break
		}
		t.Errorf("open while directories are deleted: %v", err)
	}
}

func TestCheckUploadEntry_ProbeNameTakenBySymlink(t *testing.T) {
	dir := t.TempDir()
	writeUpload(t, dir, "opinions/1_2.pdf", "pdf")
	const probeName = uploadWriteProbe + "-taken"
	if err := os.Symlink(filepath.Join(dir, "opinions", "1_2.pdf"), filepath.Join(dir, "opinions", probeName)); err != nil {
		t.Fatal(err)
	}
	root := testUploadRoot(t, dir)
	err := checkUploadEntry(root, probeName, "opinions", uploadDirEntry(t, root, ".", "opinions"), nil)
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("checkUploadEntry = %v, want %v", err, fs.ErrExist)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "opinions", "1_2.pdf")); err != nil || string(got) != "pdf" {
		t.Fatalf("symlink target changed: %q, %v", got, err)
	}
}

// uploadDirEntry returns the entry called name in directory parent of root.
func uploadDirEntry(t *testing.T, root *os.Root, parent, name string) fs.DirEntry {
	t.Helper()
	entries, err := fs.ReadDir(root.FS(), parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == name {
			return e
		}
	}
	t.Fatalf("no entry %s in %s", name, parent)
	return nil
}

func TestRemoveUploadedFiles(t *testing.T) {
	outside := t.TempDir()
	writeUpload(t, outside, "keep.pdf", "keep")
	dir := t.TempDir()
	writeUpload(t, dir, "opinions/1_2.pdf", "pdf")
	writeUpload(t, outside, "target.pdf", "target")
	if err := os.Symlink(filepath.Join(outside, "target.pdf"), filepath.Join(dir, "opinions", "link.pdf")); err != nil {
		t.Fatal(err)
	}

	removeUploadedFiles(testUploadRoot(t, dir),
		filepath.Join(dir, "opinions", "1_2.pdf"),
		filepath.Join(dir, "opinions", "missing.pdf"),
		filepath.Join(outside, "keep.pdf"),
		filepath.Join(dir, "opinions", "link.pdf"),
	)

	if _, err := os.Stat(filepath.Join(dir, "opinions", "1_2.pdf")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("file inside uploads kept: err=%v", err)
	}
	for _, kept := range []string{"keep.pdf", "target.pdf"} {
		if _, err := os.Stat(filepath.Join(outside, kept)); err != nil {
			t.Errorf("file outside uploads removed: %s: %v", kept, err)
		}
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
