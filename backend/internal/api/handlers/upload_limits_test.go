package handlers

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

type uploadCountingReader struct {
	io.Reader
	read int
}

func (r *uploadCountingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.read += n
	return n, err
}

func TestUploadRequestLimits(t *testing.T) {
	cases := []struct {
		name  string
		size  int
		extra string
		count int
		want  int
	}{
		{"maximum image", maxImageBytes, "", 1, 200},
		{"file one byte over", maxImageBytes + 1, "", 1, 400},
		{"oversized body", 8 << 20, "", 1, 413},
		{"extra file", 512, "ignored", 1, 422},
		{"duplicate file", 512, "file", 2, 422},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			r := chi.NewRouter()
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					next.ServeHTTP(w, req.WithContext(userCtx(req.Context())))
				})
			})
			RegisterUploadRoutes(humachi.New(r, huma.DefaultConfig("test", "1")), testUploadRoot(t, dir), uploadTestDB(t), nil)
			body := &bytes.Buffer{}
			mw := multipart.NewWriter(body)
			part, err := mw.CreateFormFile("file", "test.png")
			if err != nil {
				t.Fatal(err)
			}
			encoded := &bytes.Buffer{}
			if err := png.Encode(encoded, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
				t.Fatal(err)
			}
			_, _ = part.Write(append(encoded.Bytes(), bytes.Repeat([]byte("x"), max(tt.size-encoded.Len(), 0))...))
			for range tt.count - 1 {
				part, err = mw.CreateFormFile("file", "duplicate.png")
				if err != nil {
					t.Fatal(err)
				}
				_, _ = part.Write([]byte("duplicate"))
			}
			if tt.extra != "" && tt.count == 1 {
				part, err = mw.CreateFormFile(tt.extra, "extra.bin")
				if err != nil {
					t.Fatal(err)
				}
				_, _ = part.Write([]byte("extra"))
			}
			if err := mw.Close(); err != nil {
				t.Fatal(err)
			}
			reader := &uploadCountingReader{Reader: body}
			req := httptest.NewRequest(http.MethodPost, "/upload/image", reader)
			req.ContentLength = -1
			req.Header.Set("Content-Type", mw.FormDataContentType())
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			if req.MultipartForm != nil {
				defer req.MultipartForm.RemoveAll()
			}
			if rr.Code != tt.want {
				t.Fatalf("status=%d want=%d body=%s", rr.Code, tt.want, rr.Body)
			}
			if reader.read > maxUploadBodyBytes+1 {
				t.Fatalf("consumed %d bytes beyond limit", reader.read)
			}
			if tt.want != 200 {
				entries, err := os.ReadDir(dir)
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 0 {
					t.Fatal("rejected request stored files")
				}
			}
		})
	}
}

func TestUploadInputRejectsExtraValues(t *testing.T) {
	api, dir := uploadTestAPI(t)
	data := &bytes.Buffer{}
	mw := multipart.NewWriter(data)
	_ = mw.WriteField("extra", "value")
	f, err := mw.CreateFormFile("file", "a.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write(append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte("x"), 504)...))
	_ = mw.Close()
	resp := api.PostCtx(userCtx(context.Background()), "/upload/image", "Content-Type: "+mw.FormDataContentType(), data)
	if resp.Code != 422 {
		t.Fatalf("status=%d", resp.Code)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("stored rejected form")
	}
}

func TestUploadMalformedMultipart(t *testing.T) {
	api, dir := uploadTestAPI(t)
	resp := api.PostCtx(userCtx(context.Background()), "/upload/image", "Content-Type: multipart/form-data; boundary=bad", strings.NewReader("--bad\r\ninvalid"))
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("malformed request stored files")
	}
}
