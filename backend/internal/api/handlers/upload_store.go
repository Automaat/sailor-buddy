package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

const uploadTimeout = 20 * time.Second

func (h *UploadHandler) rateLimit(ctx huma.Context, next func(huma.Context)) {
	if h.db == nil {
		_ = huma.WriteErr(h.api, ctx, http.StatusServiceUnavailable, "upload storage unavailable")
		return
	}
	user := middleware.GetUser(ctx.Context())
	bounded, cancel := context.WithTimeout(ctx.Context(), uploadTimeout)
	defer cancel()
	_, err := sqlcdb.New(h.db).CountImageUploadRequest(bounded, sqlcdb.CountImageUploadRequestParams{
		UserID: user.UserID, RequestLimit: h.limits.Requests,
	})
	if errors.Is(err, sql.ErrNoRows) {
		ctx.SetHeader("Retry-After", "60")
		_ = huma.WriteErr(h.api, ctx, http.StatusTooManyRequests, "upload request limit reached; retry in one minute")
		return
	}
	if err != nil {
		slog.Error("count image upload request", "err", err)
		_ = huma.WriteErr(h.api, ctx, http.StatusServiceUnavailable, "upload storage unavailable")
		return
	}
	next(ctx)
}

func (h *UploadHandler) withAccount(ctx context.Context, userID int64, fn func(context.Context, *sql.Conn, *os.Root) error) error {
	ctx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()
	conn, err := h.db.Conn(ctx)
	if err != nil {
		return err
	}
	key := "image-upload:" + strconv.FormatInt(userID, 10)
	defer releaseUploadAccount(conn, key)
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock(hashtextextended($1, 0))", key); err != nil {
		return err
	}
	dir, err := openAccountImages(h.uploads, userID)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return fn(ctx, conn, dir)
}

func releaseUploadAccount(conn *sql.Conn, key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var unlocked bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_advisory_unlock(hashtextextended($1, 0))", key).Scan(&unlocked); err != nil || !unlocked {
		// A canceled lock acquisition has an ambiguous outcome. Discard the
		// physical connection before returning it to the pool.
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	}
	_ = conn.Close()
}

func openAccountImages(root *os.Root, userID int64) (*os.Root, error) {
	userDir := strconv.FormatInt(userID, 10)
	for _, name := range []string{userDir, userDir + "/images"} {
		if err := root.Mkdir(name, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		info, err := root.Lstat(name)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("invalid upload directory")
		}
	}
	return root.OpenRoot(userDir + "/images")
}

func (h *UploadHandler) checkQuota(ctx context.Context, q *sqlcdb.Queries, dir *os.Root, userID, incoming int64) error {
	var usedBytes, files int64
	tracked := map[string]int64{}
	rows, err := q.ListImageUploadsForQuota(ctx, sqlcdb.ListImageUploadsForQuotaParams{UserID: userID, RowLimit: h.limits.Files + 1})
	if err != nil {
		return err
	}
	for _, row := range rows {
		name := path.Base(row.Url)
		if row.Url != uploadURL(userID, name) || managedImageURL(row.Url) != row.Url {
			return errors.New("invalid tracked image path")
		}
		files++
		if files >= h.limits.Files || row.ByteSize > h.limits.Bytes-usedBytes-incoming {
			return huma.Error409Conflict("image storage quota reached")
		}
		usedBytes += row.ByteSize
		tracked[name] = row.ByteSize
	}
	return fs.WalkDir(dir.FS(), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return errors.New("unexpected upload storage entry")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		additional := info.Size()
		if reserved, ok := tracked[name]; ok {
			additional = max(0, info.Size()-reserved)
		} else {
			files++
		}
		if files >= h.limits.Files || additional > h.limits.Bytes-usedBytes-incoming {
			return huma.Error409Conflict("image storage quota reached")
		}
		usedBytes += additional
		return nil
	})
}

func uploadURL(userID int64, name string) string {
	return fmt.Sprintf("/api/uploads/%d/images/%s", userID, name)
}

func managedImageURL(value string) string {
	value = strings.TrimFunc(strings.NewReplacer("\\", "/", "\t", "", "\r", "", "\n", "").Replace(value), func(r rune) bool { return r <= 0x20 })
	value, _, _ = strings.Cut(value, "#")
	value, _, _ = strings.Cut(value, "?")
	if scheme := strings.Index(value, "://"); scheme >= 0 {
		value = value[scheme+3:]
	} else if strings.HasPrefix(value, "//") {
		value = value[2:]
	} else {
		return managedImagePath(value)
	}
	start := strings.IndexByte(value, '/')
	if start < 0 {
		return ""
	}
	return managedImagePath(value[start:])
}

func managedImagePath(value string) string {
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return ""
	}
	clean := path.Clean("/" + decoded)
	start := strings.LastIndex(clean, "/api/uploads/")
	if start < 0 {
		return ""
	}
	rel, ok := strings.CutPrefix(clean[start:], "/api/uploads/")
	parts := strings.Split(rel, "/")
	if !ok || len(parts) != 3 || parts[1] != "images" {
		return ""
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		return ""
	}
	name, extension := strings.TrimSuffix(parts[2], path.Ext(parts[2])), path.Ext(parts[2])
	if !strings.HasPrefix(name, "m-") || (extension != ".png" && extension != ".jpg" && extension != ".webp") {
		return ""
	}
	if err := uuid.Validate(strings.TrimPrefix(name, "m-")); err != nil {
		return ""
	}
	return uploadURL(id, parts[2])
}

func retainUploadImages(ctx context.Context, q sqlcdb.Querier, values ...*string) error {
	for _, value := range values {
		if value == nil {
			continue
		}
		canonical := managedImageURL(*value)
		if canonical == "" {
			continue
		}
		if _, err := q.RetainImageUpload(ctx, canonical); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return huma.Error422UnprocessableEntity("uploaded image expired or is unavailable; upload it again")
			}
			slog.Error("retain uploaded image", "err", err)
			return huma.Error503ServiceUnavailable("upload storage unavailable")
		}
	}
	return nil
}

func cleanupImages(ctx context.Context, conn *sql.Conn, dir *os.Root, userID int64) error {
	q := sqlcdb.New(conn)
	claimed, err := q.ClaimAbandonedImageUploads(ctx, userID)
	if err != nil {
		return err
	}
	for _, image := range claimed {
		name := path.Base(image.Url)
		if image.Url != uploadURL(userID, name) || managedImageURL(image.Url) != image.Url {
			return errors.New("invalid tracked image path")
		}
		if image.Owned {
			if err := dir.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
		if err := q.DeleteAbandonedImageUpload(ctx, image.Url); err != nil {
			return err
		}
	}
	return nil
}

func (h *UploadHandler) saveImage(ctx context.Context, userID int64, data []byte, extension string) (string, error) {
	var result string
	err := h.withAccount(ctx, userID, func(ctx context.Context, conn *sql.Conn, dir *os.Root) error {
		if err := cleanupImages(ctx, conn, dir, userID); err != nil {
			return err
		}
		if int64(len(data)) > h.limits.Bytes {
			return huma.Error409Conflict("image storage quota reached")
		}
		q := sqlcdb.New(conn)
		if err := h.checkQuota(ctx, q, dir, userID, int64(len(data))); err != nil {
			return err
		}
		name := "m-" + uuid.NewString() + extension
		result = uploadURL(userID, name)
		if err := q.ReserveImageUpload(ctx, sqlcdb.ReserveImageUploadParams{Url: result, UserID: userID, ByteSize: int64(len(data))}); err != nil {
			return err
		}
		return h.writeImage(ctx, q, dir, name, result, data)
	})
	return result, err
}

func (h *UploadHandler) writeImage(ctx context.Context, q *sqlcdb.Queries, dir *os.Root, name, storedURL string, data []byte) error {
	dst, err := h.createImage(dir, name)
	if err != nil {
		deleteImageReservation(ctx, q, storedURL)
		return err
	}
	keep := false
	defer func() {
		_ = dst.Close()
		if !keep {
			if err := dir.Remove(name); err == nil || errors.Is(err, fs.ErrNotExist) {
				deleteImageReservation(ctx, q, storedURL)
			}
		}
	}()
	if err := q.SetImageUploadState(ctx, sqlcdb.SetImageUploadStateParams{Url: storedURL, State: "writing"}); err != nil {
		return err
	}
	if _, err := io.Copy(dst, &uploadReader{ctx: ctx, reader: bytes.NewReader(data)}); err != nil {
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// The ready update can commit even when its acknowledgement is lost.
	// Keep the file for reference-aware cleanup on that ambiguous outcome.
	keep = true
	return q.SetImageUploadState(ctx, sqlcdb.SetImageUploadStateParams{Url: storedURL, State: "ready"})
}

func deleteImageReservation(ctx context.Context, q *sqlcdb.Queries, storedURL string) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := q.DeleteAbandonedImageUpload(cleanupCtx, storedURL); err != nil {
		slog.Warn("remove failed image reservation", "err", err)
	}
}

type uploadReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *uploadReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(data)
}
