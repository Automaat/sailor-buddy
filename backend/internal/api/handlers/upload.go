package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
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
	_ "golang.org/x/image/webp"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/config"
)

// UploadHandler stores and serves uploaded images. Every file access goes
// through uploads, so request paths cannot reach outside the upload
// directory, by ".." or by a symlink.
type UploadHandler struct {
	uploads     *os.Root
	db          *sql.DB
	limits      config.UploadLimits
	api         huma.API
	createImage func(*os.Root, string) (io.WriteCloser, error)
}

func NewUploadHandler(uploads *os.Root) *UploadHandler {
	return &UploadHandler{
		uploads: uploads, limits: config.DefaultUploadLimits(),
		createImage: func(root *os.Root, name string) (io.WriteCloser, error) {
			return root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		},
	}
}

// OpenUploadRoot creates the upload directory if needed and opens it as the
// root that confines all upload file access. It fails when the process
// cannot write every directory and file in it, so an upload volume left
// root-owned by an older image stops startup instead of failing each upload.
// The caller closes it.
func OpenUploadRoot(dir string) (*os.Root, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create upload directory: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open upload directory: %w", err)
	}
	if err := checkUploadsWritable(root); err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("upload directory %s is not writable by uid %d (chown it to this user): %w",
			dir, os.Getuid(), err)
	}
	return root, nil
}

// uploadWriteProbe prefixes the file created and removed in every upload
// directory to prove the process can add files there.
const uploadWriteProbe = ".write-probe"

// checkUploadsWritable walks uploads and fails on the first directory the
// process cannot create files in or file it cannot open for writing. Each run
// uses its own probe name, so processes sharing one upload directory can
// check it at the same time.
func checkUploadsWritable(uploads *os.Root) error {
	probeName := fmt.Sprintf("%s-%d-%s", uploadWriteProbe, os.Getpid(), rand.Text())
	return fs.WalkDir(uploads.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		return checkUploadEntry(uploads, probeName, name, d, err)
	})
}

// checkUploadEntry is the fs.WalkDirFunc of checkUploadsWritable.
//
//   - A directory gets probeName created with O_EXCL and removed again. O_EXCL
//     fails on any existing entry, symlinks included, so the probe never
//     touches a file it did not create.
//   - A regular file is opened write-only without O_TRUNC, which leaves it
//     unchanged. Files named with the probe prefix belong to other runs.
//   - Symlinks, FIFOs and other entries are skipped, so the check never
//     follows a link or blocks on a pipe.
//   - An entry another process deletes or replaces during the walk counts as
//     a pass. ENOENT always means a path component was removed, never a
//     permission failure. Any other failure is ignored when the entry is gone
//     or is no longer the same file afterwards, because macOS reports EINVAL
//     for a file created in a directory being removed. ext4 reuses inode
//     numbers at once, so a recreated directory can look unchanged; Linux
//     reports ENOENT there, which the first rule covers.
func checkUploadEntry(uploads *os.Root, probeName, name string, d fs.DirEntry, err error) error {
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	isDir := d.IsDir()
	if !isDir && (!d.Type().IsRegular() || strings.HasPrefix(d.Name(), uploadWriteProbe)) {
		return nil
	}
	before, err := uploads.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if isDir {
		err = probeUploadDir(uploads, filepath.Join(name, probeName))
	} else {
		err = openUploadForWrite(uploads, name)
	}
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	after, statErr := uploads.Lstat(name)
	if errors.Is(statErr, fs.ErrNotExist) || (statErr == nil && !os.SameFile(before, after)) {
		return nil
	}
	return err
}

// probeUploadDir creates probe with O_EXCL and removes it again. O_EXCL fails
// on any existing entry, symlinks included, so the probe never touches a file
// it did not create.
func probeUploadDir(uploads *os.Root, probe string) error {
	f, err := uploads.OpenFile(probe, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	closeErr := f.Close()
	if err := uploads.Remove(probe); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errors.Join(closeErr, err)
	}
	return closeErr
}

// openUploadForWrite opens name write-only without O_TRUNC, which leaves the
// file unchanged.
func openUploadForWrite(uploads *os.Root, name string) error {
	f, err := uploads.OpenFile(name, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	return f.Close()
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

const maxImageBytes = 5 << 20

// Multipart framing and headers have a separate, bounded allowance.
const maxUploadBodyBytes = maxImageBytes + 64<<10

type uploadImageInput struct {
	RawBody huma.MultipartFormFiles[struct {
		File huma.FormFile `form:"file" required:"true" doc:"Image file (jpeg, png or webp; max 5 MB)"`
	}]
}

func (in *uploadImageInput) Resolve(ctx huma.Context) []error {
	form, err := ctx.GetMultipartForm()
	if err == nil && len(form.File) == 1 && len(form.File["file"]) == 1 && len(form.Value) == 0 {
		return nil
	}
	if data := in.RawBody.Data(); data != nil && data.File.IsSet {
		_ = data.File.Close()
	}
	return []error{&huma.ErrorDetail{Location: "body", Message: "expected exactly one file part named file"}}
}

type uploadURLOutput struct {
	Body struct {
		URL string `json:"url"`
	}
}

// removeUploadedFiles deletes stored files through uploads. A missing file is
// fine; a path outside uploads or any other failure is logged and skipped.
func removeUploadedFiles(uploads *os.Root, paths ...string) {
	for _, stored := range paths {
		rel, err := uploadRelPath(uploads, stored)
		if err == nil {
			err = uploads.Remove(rel)
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("remove uploaded file", "path", stored, "err", err)
		}
	}
}

// RegisterUploadRoutes wires the image upload operation onto the API. Static
// file serving (GET /uploads/*) stays on chi as it is not an API operation.
func RegisterUploadRoutes(api huma.API, uploads *os.Root, db *sql.DB, limits *config.UploadLimits) {
	h := NewUploadHandler(uploads)
	h.db = db
	if limits != nil {
		h.limits = *limits
	}
	registerUploadRoutes(api, h)
}

func registerUploadRoutes(api huma.API, h *UploadHandler) {
	h.api = api
	huma.Register(api, huma.Operation{
		OperationID: "upload-image", Method: http.MethodPost, Path: "/upload/image",
		Summary: "Upload an image", Tags: []string{"Uploads"},
		MaxBodyBytes: maxUploadBodyBytes,
		Middlewares:  huma.Middlewares{h.rateLimit},
	}, h.uploadImage)
	huma.Register(api, huma.Operation{
		OperationID: "cleanup-images", Method: http.MethodPost, Path: "/upload/images/cleanup",
		Summary: "Remove my expired unattached images", Tags: []string{"Uploads"},
		DefaultStatus: http.StatusNoContent, Middlewares: huma.Middlewares{h.rateLimit},
	}, h.cleanup)
}

func (h *UploadHandler) uploadImage(ctx context.Context, in *uploadImageInput) (*uploadURLOutput, error) {
	user := middleware.GetUser(ctx)
	file := in.RawBody.Data().File
	if !file.IsSet {
		return nil, huma.Error400BadRequest("missing file field")
	}
	defer func() { _ = file.Close() }()
	if file.Size > maxImageBytes {
		return nil, huma.Error400BadRequest("file too large (max 5MB)")
	}

	data, extension, err := validatedImage(file)
	if err != nil {
		return nil, err
	}
	url, err := h.saveImage(ctx, user.UserID, data, extension)
	if err != nil {
		return nil, uploadStorageError(err)
	}
	out := &uploadURLOutput{}
	out.Body.URL = url
	return out, nil
}

func validatedImage(reader io.Reader) (data []byte, extension string, err error) {
	data, err = io.ReadAll(io.LimitReader(reader, maxImageBytes+1))
	if err != nil || len(data) > maxImageBytes {
		return nil, "", huma.Error400BadRequest("cannot read image or image exceeds 5 MB")
	}
	header, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png" && format != "webp") {
		return nil, "", huma.Error400BadRequest("invalid image; allowed: jpeg, png, webp")
	}
	if header.Width < 1 || header.Height < 1 || header.Width > 4096 || header.Height > 4096 || int64(header.Width)*int64(header.Height) > 4_000_000 {
		return nil, "", huma.Error400BadRequest("image exceeds 4096 pixels per side or four million pixels")
	}
	decoded, decodedFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil || decodedFormat != format || decoded.Bounds().Dx() != header.Width || decoded.Bounds().Dy() != header.Height {
		return nil, "", huma.Error400BadRequest("invalid image content")
	}
	return data, allowedMIME["image/"+format], nil
}

func uploadStorageError(err error) error {
	if _, ok := errors.AsType[huma.StatusError](err); ok {
		return err
	}
	slog.Error("image storage failure", "err", err)
	return huma.Error503ServiceUnavailable("upload storage unavailable")
}

func (h *UploadHandler) cleanup(ctx context.Context, _ *struct{}) (*noContentOutput, error) {
	userID := middleware.GetUser(ctx).UserID
	if err := h.withAccount(ctx, userID, func(bounded context.Context, conn *sql.Conn, dir *os.Root) error {
		return cleanupImages(bounded, conn, dir, userID)
	}); err != nil {
		return nil, uploadStorageError(err)
	}
	return &noContentOutput{}, nil
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

	userDir := strconv.FormatInt(user.UserID, 10)
	userInfo, err := h.uploads.Lstat(userDir)
	if err == nil && (!userInfo.IsDir() || userInfo.Mode()&os.ModeSymlink != 0) {
		respondError(w, http.StatusBadRequest, "invalid path")
		return
	}
	ownerRoot, err := h.uploads.OpenRoot(userDir)
	if err != nil {
		respondError(w, http.StatusNotFound, "file not found")
		return
	}
	defer func() { _ = ownerRoot.Close() }()
	f, err := ownerRoot.Open(strings.TrimPrefix(clean, userPrefix))
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
