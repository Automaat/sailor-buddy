package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/png"
	"io"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/config"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type imageStorageFixture struct {
	conn        *sql.DB
	h           *UploadHandler
	api         humatest.TestAPI
	dir         string
	data        []byte
	contentType string
}

func newImageStorageFixture(t *testing.T, limits config.UploadLimits) *imageStorageFixture {
	t.Helper()
	f := &imageStorageFixture{conn: uploadTestDB(t), dir: t.TempDir()}
	f.h = NewUploadHandler(testUploadRoot(t, f.dir))
	f.h.db, f.h.limits = f.conn, limits
	_, f.api = humatest.New(t)
	registerUploadRoutes(f.api, f.h)
	body, ct := multipartBody(t, "image.png", func(w io.Writer) {
		if err := png.Encode(w, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
			t.Fatal(err)
		}
	})
	f.data, f.contentType = body.Bytes(), ct
	return f
}

func (f *imageStorageFixture) upload(userID int64) *httptest.ResponseRecorder {
	return f.api.PostCtx(callerCtx(userID, "admin"), "/upload/image", "Content-Type: "+f.contentType, bytes.NewReader(f.data))
}

func storedURL(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	if response.Code != 200 {
		t.Fatalf("upload: %d %s", response.Code, response.Body)
	}
	var result struct {
		URL string `json:"url"`
	}
	decodeBody(t, response.Body.Bytes(), &result)
	return result.URL
}

func TestImageStorage_ByteAndFileBoundaries(t *testing.T) {
	for _, name := range []string{"files", "bytes"} {
		t.Run(name, func(t *testing.T) {
			limits := config.DefaultUploadLimits()
			limits.Files = 2
			if name == "bytes" {
				limits.Files = 100
			}
			f := newImageStorageFixture(t, limits)
			first := storedURL(t, f.upload(1))
			info, err := os.Stat(filepath.Join(f.dir, strings.TrimPrefix(first, "/api/uploads/")))
			if err != nil {
				t.Fatal(err)
			}
			if name == "bytes" {
				f.h.limits.Bytes = info.Size() * 2
			}
			storedURL(t, f.upload(1))
			if response := f.upload(1); response.Code != 409 {
				t.Fatalf("over quota: %d %s", response.Code, response.Body)
			}
			entries, err := os.ReadDir(filepath.Join(f.dir, "1/images"))
			if err != nil || len(entries) != 2 {
				t.Fatalf("stored %d files: %v", len(entries), err)
			}
		})
	}
}

func TestImageStorage_LegacyFilesCountButAreNeverCleaned(t *testing.T) {
	limits := config.DefaultUploadLimits()
	limits.Files = 1
	f := newImageStorageFixture(t, limits)
	writeUpload(t, f.dir, "1/images/legacy.png", "existing image")
	if response := f.upload(1); response.Code != 409 {
		t.Fatalf("legacy quota: %d %s", response.Code, response.Body)
	}
	if response := f.api.PostCtx(callerCtx(1, "admin"), "/upload/images/cleanup"); response.Code != 204 {
		t.Fatalf("cleanup: %d", response.Code)
	}
	data, err := os.ReadFile(filepath.Join(f.dir, "1/images/legacy.png"))
	if err != nil || string(data) != "existing image" {
		t.Fatalf("legacy changed: %q %v", data, err)
	}
}

func TestImageStorage_ConcurrentProcessesShareQuota(t *testing.T) {
	for _, quota := range []string{"files", "bytes"} {
		t.Run(quota, func(t *testing.T) {
			f := newImageStorageFixture(t, config.DefaultUploadLimits())
			var databaseName string
			if err := f.conn.QueryRow("SELECT current_database()").Scan(&databaseName); err != nil {
				t.Fatal(err)
			}
			dsn, err := url.Parse(os.Getenv("SAILOR_TEST_DATABASE_URL"))
			if err != nil {
				t.Fatal(err)
			}
			dsn.Path = "/" + databaseName
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			results := make(chan int, 8)
			for range 8 {
				go func() {
					cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestImageStorage_QuotaProcess$")
					cmd.Env = append(os.Environ(), "SAILOR_IMAGE_TEST_DSN="+dsn.String(), "SAILOR_IMAGE_TEST_DIR="+f.dir, "SAILOR_IMAGE_TEST_QUOTA="+quota)
					output, err := cmd.CombinedOutput()
					code := 0
					if err != nil {
						if exit, ok := errors.AsType[*exec.ExitError](err); ok {
							code = exit.ExitCode()
						} else {
							code = -1
						}
					}
					if code != 0 && code != 10 {
						t.Logf("upload process %d: %s", code, output)
					}
					results <- code
				}()
			}
			counts := map[int]int{}
			for range 8 {
				counts[<-results]++
			}
			if counts[0] != 3 || counts[10] != 5 || len(counts) != 2 {
				t.Fatalf("process outcomes: %v", counts)
			}
			entries, err := os.ReadDir(filepath.Join(f.dir, "1/images"))
			if err != nil || len(entries) != 3 {
				t.Fatalf("stored %d files: %v", len(entries), err)
			}
		})
	}
}

func TestImageStorage_QuotaProcess(t *testing.T) {
	dsn := os.Getenv("SAILOR_IMAGE_TEST_DSN")
	if dsn == "" {
		t.Skip("subprocess helper")
	}
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(os.Getenv("SAILOR_IMAGE_TEST_DIR"))
	if err != nil {
		t.Fatal(err)
	}
	h := NewUploadHandler(root)
	h.db, h.limits.Files = conn, 3
	if os.Getenv("SAILOR_IMAGE_TEST_QUOTA") == "bytes" {
		h.limits.Files, h.limits.Bytes = 100, 1536
	}
	_, err = h.saveImage(t.Context(), 1, bytes.Repeat([]byte("x"), 512), ".png")
	_ = root.Close()
	_ = conn.Close()
	if err != nil {
		var status huma.StatusError
		if errors.As(err, &status) && status.GetStatus() == 409 {
			os.Exit(10)
		}
		t.Fatal(err)
	}
}

func TestImageStorage_RateLimitCountsMalformedBodiesAndResets(t *testing.T) {
	limits := config.DefaultUploadLimits()
	limits.Requests = 3
	f := newImageStorageFixture(t, limits)
	var wg sync.WaitGroup
	statuses := make(chan int, 8)
	for range 8 {
		wg.Go(func() {
			r := f.api.PostCtx(callerCtx(1, "admin"), "/upload/image", "Content-Type: multipart/form-data; boundary=broken", strings.NewReader("invalid"))
			statuses <- r.Code
		})
	}
	wg.Wait()
	close(statuses)
	counts := map[int]int{}
	for code := range statuses {
		counts[code]++
	}
	if counts[422] != 3 || counts[429] != 5 {
		t.Fatalf("rate outcomes: %v", counts)
	}
	response := f.upload(1)
	if response.Code != 429 || response.Header().Get("Retry-After") != "60" {
		t.Fatalf("rate reply: %d %v", response.Code, response.Header())
	}
	if _, err := sqlcdb.New(f.conn).CreateUser(t.Context(), sqlcdb.CreateUserParams{Email: "independent@example.test", Name: "Other"}); err != nil {
		t.Fatal(err)
	}
	storedURL(t, f.upload(2))
	if _, err := f.conn.Exec("UPDATE image_upload_requests SET window_start=clock_timestamp()-interval '61 seconds'"); err != nil {
		t.Fatal(err)
	}
	storedURL(t, f.upload(1))
}

func TestImageStorage_ReservationsCountWhenFilesystemViewIsMissing(t *testing.T) {
	limits := config.DefaultUploadLimits()
	limits.Files = 1
	f := newImageStorageFixture(t, limits)
	imageURL := storedURL(t, f.upload(1))
	if err := os.Remove(filepath.Join(f.dir, strings.TrimPrefix(imageURL, "/api/uploads/"))); err != nil {
		t.Fatal(err)
	}
	if response := f.upload(1); response.Code != 409 {
		t.Fatalf("missing file bypassed shared quota: %d %s", response.Code, response.Body)
	}
}

func TestImageStorage_RetriesCleanupAndProtectsUnconfirmedFiles(t *testing.T) {
	f := newImageStorageFixture(t, config.DefaultUploadLimits())
	q := sqlcdb.New(f.conn)
	const name = "m-7ecb1808-e9a9-4bd3-b0cf-d46902333c72.png"
	imageURL := uploadURL(1, name)
	writeUpload(t, f.dir, "1/images/"+name, "unconfirmed existing file")
	if err := q.ReserveImageUpload(t.Context(), sqlcdb.ReserveImageUploadParams{Url: imageURL, UserID: 1, ByteSize: 32}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.conn.Exec("UPDATE image_uploads SET created_at=clock_timestamp()-interval '25 hours'"); err != nil {
		t.Fatal(err)
	}
	if response := f.api.PostCtx(callerCtx(1, "admin"), "/upload/images/cleanup"); response.Code != 204 {
		t.Fatalf("cleanup: %d", response.Code)
	}
	data, err := os.ReadFile(filepath.Join(f.dir, "1/images/"+name))
	if err != nil || string(data) != "unconfirmed existing file" {
		t.Fatalf("unconfirmed file removed: %q %v", data, err)
	}
	for _, state := range []string{"writing", "ready", "deleting"} {
		t.Run(state, func(t *testing.T) {
			f := newImageStorageFixture(t, config.DefaultUploadLimits())
			q := sqlcdb.New(f.conn)
			writeUpload(t, f.dir, "1/images/"+name, "interrupted data")
			if err := q.ReserveImageUpload(t.Context(), sqlcdb.ReserveImageUploadParams{Url: imageURL, UserID: 1, ByteSize: 512}); err != nil {
				t.Fatal(err)
			}
			if err := q.SetImageUploadState(t.Context(), sqlcdb.SetImageUploadStateParams{Url: imageURL, State: "writing"}); err != nil {
				t.Fatal(err)
			}
			if err := q.SetImageUploadState(t.Context(), sqlcdb.SetImageUploadStateParams{Url: imageURL, State: state}); err != nil {
				t.Fatal(err)
			}
			if _, err := f.conn.Exec("UPDATE image_uploads SET created_at=clock_timestamp()-interval '25 hours'"); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if response := f.api.PostCtx(callerCtx(1, "admin"), "/upload/images/cleanup"); response.Code != 204 {
					t.Fatalf("cleanup retry: %d %s", response.Code, response.Body)
				}
			}
			if _, err := os.Stat(filepath.Join(f.dir, "1/images/"+name)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("interrupted image retained: %v", err)
			}
		})
	}
}

func TestImageStorage_CleanupKeepsReferencedRecentAndOtherOwners(t *testing.T) {
	f := newImageStorageFixture(t, config.DefaultUploadLimits())
	q := sqlcdb.New(f.conn)
	if _, err := q.CreateUser(t.Context(), sqlcdb.CreateUserParams{Email: "other@example.test", Name: "Other"}); err != nil {
		t.Fatal(err)
	}
	retained := storedURL(t, f.upload(1))
	abandoned := storedURL(t, f.upload(1))
	other := storedURL(t, f.upload(2))
	_, api := humatest.New(t)
	RegisterCruiseRoutes(api, q, f.conn)
	response := api.PostCtx(callerCtx(1, "admin"), "/cruises", asDBEdition(t, f.conn, map[string]any{"name": "Attached", "image_photo_url": retained}))
	if response.Code != 201 {
		t.Fatalf("attach: %d %s", response.Code, response.Body)
	}
	if _, err := f.conn.Exec("UPDATE image_uploads SET created_at=clock_timestamp()-interval '25 hours'"); err != nil {
		t.Fatal(err)
	}
	recent := storedURL(t, f.upload(1)) // also cleans this owner's abandoned upload
	if response := f.api.PostCtx(callerCtx(1, "admin"), "/upload/images/cleanup"); response.Code != 204 {
		t.Fatalf("cleanup: %d %s", response.Code, response.Body)
	}
	for _, imageURL := range []string{retained, recent, other} {
		if _, err := os.Stat(filepath.Join(f.dir, strings.TrimPrefix(imageURL, "/api/uploads/"))); err != nil {
			t.Fatalf("protected %s: %v", imageURL, err)
		}
	}
	if _, err := os.Stat(filepath.Join(f.dir, strings.TrimPrefix(abandoned, "/api/uploads/"))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("abandoned remains: %v", err)
	}
	response = api.PostCtx(callerCtx(1, "admin"), "/cruises", asDBEdition(t, f.conn, map[string]any{"name": "Expired", "image_photo_url": abandoned}))
	if response.Code != 422 {
		t.Fatalf("attach deleted image: %d %s", response.Code, response.Body)
	}
}

type brokenImageWriter struct {
	io.WriteCloser
	failClose bool
	cancel    context.CancelFunc
}

func (w *brokenImageWriter) Write(data []byte) (int, error) {
	if w.cancel != nil {
		n, err := w.WriteCloser.Write(data)
		w.cancel()
		return n, err
	}
	if w.failClose {
		return w.WriteCloser.Write(data)
	}
	n, _ := w.WriteCloser.Write(data[:min(8, len(data))])
	return n, io.ErrUnexpectedEOF
}

func (w *brokenImageWriter) Close() error {
	err := w.WriteCloser.Close()
	if w.failClose {
		return io.ErrClosedPipe
	}
	return err
}

func TestImageStorage_FailedAndInterruptedWritesLeaveNoFile(t *testing.T) {
	for _, scenario := range []string{"write", "close", "cancel"} {
		t.Run(scenario, func(t *testing.T) {
			f := newImageStorageFixture(t, config.DefaultUploadLimits())
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			f.h.createImage = func(root *os.Root, name string) (io.WriteCloser, error) {
				file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
				if err != nil {
					return nil, err
				}
				w := &brokenImageWriter{WriteCloser: file, failClose: scenario == "close"}
				if scenario == "cancel" {
					w.cancel = cancel
				}
				return w, nil
			}
			if _, err := f.h.saveImage(ctx, 1, bytes.Repeat([]byte("x"), 1024), ".png"); err == nil {
				t.Fatal("expected interrupted save")
			}
			entries, err := os.ReadDir(filepath.Join(f.dir, "1/images"))
			if err != nil || len(entries) != 0 {
				t.Fatalf("failed write retained %d files: %v", len(entries), err)
			}
		})
	}
}

func TestImageStorage_CanceledCreationFailureReleasesQuota(t *testing.T) {
	limits := config.DefaultUploadLimits()
	limits.Files = 1
	f := newImageStorageFixture(t, limits)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	create := f.h.createImage
	f.h.createImage = func(*os.Root, string) (io.WriteCloser, error) { cancel(); return nil, os.ErrPermission }
	if _, err := f.h.saveImage(ctx, 1, []byte("validated image"), ".png"); err == nil {
		t.Fatal("expected failed creation")
	}
	f.h.createImage = create
	storedURL(t, f.upload(1))
}

func TestValidatedImage_RejectsSignaturesTruncationAndDimensions(t *testing.T) {
	valid := &bytes.Buffer{}
	if err := png.Encode(valid, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	header := append([]byte(nil), valid.Bytes()...)
	binary.BigEndian.PutUint32(header[16:20], 5000)
	binary.BigEndian.PutUint32(header[20:24], 1)
	binary.BigEndian.PutUint32(header[29:33], crc32.ChecksumIEEE(header[12:29]))
	pixels := append([]byte(nil), header...)
	binary.BigEndian.PutUint32(pixels[16:20], 2001)
	binary.BigEndian.PutUint32(pixels[20:24], 2000)
	binary.BigEndian.PutUint32(pixels[29:33], crc32.ChecksumIEEE(pixels[12:29]))
	cases := []struct {
		name string
		data []byte
	}{
		{"signature only", append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte("x"), 504)...)},
		{"truncated pixels", valid.Bytes()[:len(valid.Bytes())-20]},
		{"oversized dimensions", header},
		{"oversized pixel count", pixels},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := validatedImage(bytes.NewReader(tt.data)); err == nil {
				t.Fatal("accepted invalid image")
			}
		})
	}
}

func TestValidatedImage_SupportsWebP(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	_, extension, err := validatedImage(bytes.NewReader(data))
	if err != nil || extension != ".webp" {
		t.Fatalf("webp: %q %v", extension, err)
	}
}
