package handlers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/config"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func TestManagedImageURL_ProtectsBrowserURLForms(t *testing.T) {
	const canonical = "/api/uploads/1/images/m-7ecb1808-e9a9-4bd3-b0cf-d46902333c72.png"
	for _, value := range []string{
		canonical, "https://club.test" + canonical + "?cache=1#image", "  " + canonical + "\n",
		"../.." + canonical, strings.ReplaceAll(canonical, "/", "\\"),
		strings.Replace(canonical, "/m-", "/%6d-", 1), "/prefix" + canonical,
		"\x01" + canonical + "\x02", canonical + "#%", "https://user%zz@club.test" + canonical,
		"//club.test" + canonical + "?invalid=%",
		"/api/uploads/1/images/unused/../m-7ecb1808-e9a9-4bd3-b0cf-d46902333c72.png",
	} {
		t.Run(value, func(t *testing.T) {
			if got := managedImageURL(value); got != canonical {
				t.Fatalf("canonical: %q", got)
			}
		})
	}
}

func TestImageReferences_AllResourceFieldsSurviveCleanup(t *testing.T) {
	f := newImageStorageFixture(t, config.DefaultUploadLimits())
	_, api := humatest.New(t)
	q := sqlcdb.New(f.conn)
	RegisterCruiseRoutes(api, q, f.conn)
	RegisterTripRoutes(api, q, f.conn)
	RegisterVoyageRoutes(api, q)
	var parentID int64
	if err := f.conn.QueryRow("INSERT INTO cruises(name,kind) VALUES ('Parent','sea') RETURNING id").Scan(&parentID); err != nil {
		t.Fatal(err)
	}
	var images []string
	for _, resource := range []string{"cruises", "trips", "voyages"} {
		body := map[string]any{"name": "With images"}
		if resource == "cruises" {
			body["kind"] = "sea"
		} else {
			body["cruise_id"] = parentID
		}
		for _, field := range []string{"image_logo_url", "image_photo_url", "image_route_url"} {
			imageURL := storedURL(t, f.upload(1))
			images = append(images, imageURL)
			body[field] = imageURL
		}
		response := api.PostCtx(callerCtx(1, "admin"), "/"+resource, body)
		if response.Code != 201 {
			t.Fatalf("create %s: %d %s", resource, response.Code, response.Body)
		}
		var record struct {
			ID int64 `json:"id"`
		}
		decodeBody(t, response.Body.Bytes(), &record)
		for _, field := range []string{"image_logo_url", "image_photo_url", "image_route_url"} {
			imageURL := storedURL(t, f.upload(1))
			images = append(images, imageURL)
			body[field] = imageURL
		}
		response = api.PutCtx(callerCtx(1, "admin"), "/"+resource+"/"+strconv.FormatInt(record.ID, 10), body)
		if response.Code != 204 {
			t.Fatalf("update %s: %d %s", resource, response.Code, response.Body)
		}
	}
	if _, err := f.conn.Exec("UPDATE image_uploads SET created_at=clock_timestamp()-interval '25 hours'"); err != nil {
		t.Fatal(err)
	}
	response := f.api.PostCtx(callerCtx(1, "admin"), "/upload/images/cleanup")
	if response.Code != 204 {
		t.Fatalf("cleanup: %d %s", response.Code, response.Body)
	}
	for _, imageURL := range images {
		if _, err := os.Stat(filepath.Join(f.dir, strings.TrimPrefix(imageURL, "/api/uploads/"))); err != nil {
			t.Fatalf("retained image: %v", err)
		}
	}
}

func TestImageReferences_ConcurrentCleanupAndAttach(t *testing.T) {
	for _, winner := range []string{"attach", "cleanup"} {
		t.Run(winner, func(t *testing.T) {
			f := newImageStorageFixture(t, config.DefaultUploadLimits())
			imageURL := storedURL(t, f.upload(1))
			if _, err := f.conn.Exec("UPDATE image_uploads SET created_at=clock_timestamp()-interval '25 hours'"); err != nil {
				t.Fatal(err)
			}
			tx, err := f.conn.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback() }()
			q := sqlcdb.New(tx)
			if winner == "attach" {
				if _, err := q.RetainImageUpload(t.Context(), imageURL); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := q.ClaimAbandonedImageUploads(t.Context(), 1); err != nil {
					t.Fatal(err)
				}
			}
			_, api := humatest.New(t)
			RegisterCruiseRoutes(api, sqlcdb.New(f.conn), f.conn)
			ctx, cancel := context.WithTimeout(callerCtx(1, "admin"), 5*time.Second)
			defer cancel()
			result := make(chan int, 1)
			if winner == "attach" {
				go func() { result <- f.api.PostCtx(ctx, "/upload/images/cleanup").Code }()
				if status := <-result; status != 204 {
					t.Fatalf("cleanup blocked on attaching row: %d", status)
				}
			} else {
				go func() {
					result <- api.PostCtx(ctx, "/cruises", map[string]any{"name": "Race", "kind": "sea", "image_logo_url": imageURL}).Code
				}()
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			if winner == "cleanup" {
				if status := <-result; status != 422 {
					t.Fatalf("attached deleting image: %d", status)
				}
			}
			if status := f.api.PostCtx(callerCtx(1, "admin"), "/upload/images/cleanup").Code; status != 204 {
				t.Fatalf("cleanup: %d", status)
			}
			_, err = os.Stat(filepath.Join(f.dir, strings.TrimPrefix(imageURL, "/api/uploads/")))
			if winner == "attach" && err != nil {
				t.Fatalf("attached file removed: %v", err)
			}
			if winner == "cleanup" && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("abandoned file kept: %v", err)
			}
		})
	}
}
