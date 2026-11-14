package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// keysetCase is one paged list endpoint: its table, how to register it and
// the rows to seed. Seeds tie on the leading sort keys and mix in NULLs, so
// page boundaries fall inside ties and between NULL and non-NULL keys.
type keysetCase struct {
	name     string
	table    string
	path     string
	register func(api huma.API, q sqlcdb.Querier, conn *sql.DB)
	seed     func(t *testing.T, conn *sql.DB, user int64)
}

func execSeed(t *testing.T, conn *sql.DB, stmt string, args ...any) {
	t.Helper()
	if _, err := conn.ExecContext(context.Background(), stmt, args...); err != nil {
		t.Fatalf("seed %q: %v", stmt, err)
	}
}

func keysetCases() []keysetCase {
	names := []string{"Bravo", "Alfa", "Bravo", "Charlie", "Alfa", "Bravo", "Alfa"}
	dates := []any{"2026-05-01", nil, "2026-05-01", "2025-01-01", nil, "2026-05-01", "2027-02-02"}
	years := []any{2026, nil, 2026, 2025, 2026, nil, 2026}
	return []keysetCase{
		{
			name: "crew", table: "crew_members", path: "/crew",
			register: func(api huma.API, q sqlcdb.Querier, conn *sql.DB) { RegisterCrewRoutes(api, q, conn, nil) },
			seed: func(t *testing.T, conn *sql.DB, _ int64) {
				t.Helper()
				for _, n := range names {
					execSeed(t, conn, "INSERT INTO crew_members (full_name) VALUES ($1)", n)
				}
			},
		},
		{
			name: "series", table: "cruise_series", path: "/series",
			register: func(api huma.API, q sqlcdb.Querier, _ *sql.DB) { RegisterSeriesRoutes(api, q) },
			seed: func(t *testing.T, conn *sql.DB, _ int64) {
				t.Helper()
				for _, n := range names {
					execSeed(t, conn, "INSERT INTO cruise_series (name) VALUES ($1)", n)
				}
			},
		},
		{
			name: "yachts", table: "yachts", path: "/yachts",
			register: func(api huma.API, q sqlcdb.Querier, _ *sql.DB) { RegisterYachtRoutes(api, q) },
			seed: func(t *testing.T, conn *sql.DB, _ int64) {
				t.Helper()
				for _, n := range names {
					execSeed(t, conn, "INSERT INTO yachts (name) VALUES ($1)", n)
				}
			},
		},
		{
			name: "cruises", table: "cruises", path: "/cruises",
			register: func(api huma.API, q sqlcdb.Querier, conn *sql.DB) { RegisterCruiseRoutes(api, q, conn) },
			seed: func(t *testing.T, conn *sql.DB, _ int64) {
				t.Helper()
				for _, d := range dates {
					execSeed(t, conn, "INSERT INTO cruises (name, embark_date) VALUES ('C', $1)", d)
				}
			},
		},
		{
			name: "trips", table: "trips", path: "/trips",
			register: func(api huma.API, q sqlcdb.Querier, conn *sql.DB) { RegisterTripRoutes(api, q, conn) },
			seed: func(t *testing.T, conn *sql.DB, _ int64) {
				t.Helper()
				execSeed(t, conn, "INSERT INTO cruises (name) VALUES ('Parent')")
				for _, d := range dates {
					execSeed(t, conn, "INSERT INTO trips (cruise_id, name, embark_date) "+
						"VALUES ((SELECT min(id) FROM cruises), 'T', $1)", d)
				}
			},
		},
		{
			name: "voyages", table: "voyages", path: "/voyages",
			register: func(api huma.API, q sqlcdb.Querier, _ *sql.DB) { RegisterVoyageRoutes(api, q) },
			seed: func(t *testing.T, conn *sql.DB, _ int64) {
				t.Helper()
				execSeed(t, conn, "INSERT INTO cruises (name) VALUES ('Parent')")
				for i, d := range dates {
					execSeed(t, conn, "INSERT INTO voyages (cruise_id, name, year, embark_date) "+
						"VALUES ((SELECT min(id) FROM cruises), 'V', $1, $2)", years[i], d)
				}
			},
		},
		{
			name: "trainings", table: "trainings", path: "/trainings",
			register: func(api huma.API, q sqlcdb.Querier, _ *sql.DB) { RegisterTrainingRoutes(api, q) },
			seed: func(t *testing.T, conn *sql.DB, user int64) {
				t.Helper()
				for _, d := range dates {
					execSeed(t, conn, "INSERT INTO trainings (user_id, name, date) VALUES ($1, 'K', $2)", user, d)
				}
			},
		},
	}
}

type idPage struct {
	Items []struct {
		ID int64 `json:"id"`
	} `json:"items"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor"`
}

func getIDPage(t *testing.T, api humatest.TestAPI, ctx context.Context, path string) idPage {
	t.Helper()
	resp := api.GetCtx(ctx, path)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET %s: got %d; body=%s", path, resp.Code, resp.Body)
	}
	var page idPage
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return page
}

func pageIDs(p idPage) []int64 {
	ids := make([]int64, len(p.Items))
	for i, it := range p.Items {
		ids[i] = it.ID
	}
	return ids
}

// TestListPaging_CursorSurvivesDeletes walks every paged list two rows at a
// time with next_cursor and deletes both rows of the first page before the
// second request. Offset paging would then skip two rows; the cursor walk
// must still return every remaining row once, in the full list's order.
func TestListPaging_CursorSurvivesDeletes(t *testing.T) {
	conn := lockTestDB(t)
	q := sqlcdb.New(conn)
	user, err := q.CreateUser(context.Background(), sqlcdb.CreateUserParams{Email: "pager@example.com"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	ctx := memberCtx(user.ID)

	for _, tc := range keysetCases() {
		t.Run(tc.name, func(t *testing.T) {
			execSeed(t, conn, "TRUNCATE "+tc.table+" RESTART IDENTITY CASCADE")
			tc.seed(t, conn, user.ID)
			_, api := humatest.New(t)
			tc.register(api, q, conn)

			first := getIDPage(t, api, ctx, tc.path+"?limit=2")
			if !first.HasMore || first.NextCursor == "" || len(first.Items) != 2 {
				t.Fatalf("first page = %+v, want two rows and a cursor", first)
			}
			deleted := pageIDs(first)
			for _, id := range deleted {
				execSeed(t, conn, "DELETE FROM "+tc.table+" WHERE id = $1", id)
			}

			var walked []int64
			page := first
			for requests := 1; page.HasMore; requests++ {
				if requests > 10 {
					t.Fatalf("walk did not end; walked %v", walked)
				}
				page = getIDPage(t, api, ctx, fmt.Sprintf("%s?limit=2&after=%s", tc.path, url.QueryEscape(page.NextCursor)))
				walked = append(walked, pageIDs(page)...)
			}

			want := pageIDs(getIDPage(t, api, ctx, tc.path+"?limit=100"))
			if len(want) != 5 {
				t.Fatalf("full list after deletes = %v, want 5 rows", want)
			}
			if !slices.Equal(walked, want) {
				t.Fatalf("cursor walk after deleting %v = %v, want %v", deleted, walked, want)
			}
		})
	}
}

// TestListPaging_CursorMatchesOffsetOrder checks that a cursor walk of an
// unchanged list returns the same rows, in the same order and with the same
// number of requests, as offset paging.
func TestListPaging_CursorMatchesOffsetOrder(t *testing.T) {
	conn := lockTestDB(t)
	q := sqlcdb.New(conn)
	user, err := q.CreateUser(context.Background(), sqlcdb.CreateUserParams{Email: "pager@example.com"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	ctx := memberCtx(user.ID)

	for _, tc := range keysetCases() {
		t.Run(tc.name, func(t *testing.T) {
			execSeed(t, conn, "TRUNCATE "+tc.table+" RESTART IDENTITY CASCADE")
			tc.seed(t, conn, user.ID)
			_, api := humatest.New(t)
			tc.register(api, q, conn)

			var byOffset []int64
			offsetRequests := 0
			for offset := 0; ; offset += 2 {
				offsetRequests++
				p := getIDPage(t, api, ctx, fmt.Sprintf("%s?limit=2&offset=%d", tc.path, offset))
				byOffset = append(byOffset, pageIDs(p)...)
				if !p.HasMore {
					break
				}
			}

			var byCursor []int64
			cursorRequests := 0
			path := tc.path + "?limit=2"
			for {
				cursorRequests++
				p := getIDPage(t, api, ctx, path)
				byCursor = append(byCursor, pageIDs(p)...)
				if !p.HasMore {
					break
				}
				path = fmt.Sprintf("%s?limit=2&after=%s", tc.path, url.QueryEscape(p.NextCursor))
			}

			if len(byOffset) != 7 || !slices.Equal(byCursor, byOffset) {
				t.Fatalf("cursor walk %v, offset walk %v; want the same 7 rows", byCursor, byOffset)
			}
			if cursorRequests != offsetRequests || cursorRequests != 4 {
				t.Fatalf("requests: cursor %d, offset %d; want 4 each for 7 rows", cursorRequests, offsetRequests)
			}
		})
	}
}
