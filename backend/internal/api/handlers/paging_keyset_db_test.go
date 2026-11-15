package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
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
	// caller builds the requesting context; nil means a plain member.
	caller func(user int64) context.Context
}

// callerFor is the context tc's list is read with.
func (tc keysetCase) callerFor(user int64) context.Context {
	if tc.caller != nil {
		return tc.caller(user)
	}
	return memberCtx(user)
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
			caller:   captainGroupCtx,
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
				for range dates {
					execSeed(t, conn, "INSERT INTO cruises (name, published_at) VALUES ('C', now())")
				}
				execSeed(t, conn, "INSERT INTO cruises (name) VALUES ('Hidden')")
			},
		},
		{
			name: "trips", table: "trips", path: "/trips",
			register: func(api huma.API, q sqlcdb.Querier, conn *sql.DB) { RegisterTripRoutes(api, q, conn) },
			seed: func(t *testing.T, conn *sql.DB, _ int64) {
				t.Helper()
				execSeed(t, conn, "INSERT INTO cruises (name, published_at) VALUES ('Parent', now())")
				for _, d := range dates {
					execSeed(t, conn, "INSERT INTO trips (cruise_id, name, embark_date, published_at) "+
						"VALUES ((SELECT min(id) FROM cruises), 'T', $1, now())", d)
				}
				execSeed(t, conn, "INSERT INTO trips (cruise_id, name, embark_date) "+
					"VALUES ((SELECT min(id) FROM cruises), 'Hidden', $1)", dates[3])
				execSeed(t, conn, "INSERT INTO cruises (name) VALUES ('Hidden parent')")
				execSeed(t, conn, "INSERT INTO trips (cruise_id, name, embark_date, published_at) "+
					"VALUES ((SELECT max(id) FROM cruises), 'In hidden', $1, now())", dates[3])
			},
		},
		{
			name: "voyages", table: "voyages", path: "/voyages",
			register: func(api huma.API, q sqlcdb.Querier, _ *sql.DB) { RegisterVoyageRoutes(api, q) },
			seed: func(t *testing.T, conn *sql.DB, _ int64) {
				t.Helper()
				execSeed(t, conn, "INSERT INTO cruises (name, published_at) VALUES ('Parent', now())")
				for i, d := range dates {
					execSeed(t, conn, "INSERT INTO voyages (cruise_id, name, year, embark_date, published_at) "+
						"VALUES ((SELECT min(id) FROM cruises), 'V', $1, $2, now())", years[i], d)
				}
				execSeed(t, conn, "INSERT INTO voyages (cruise_id, name, year, embark_date) "+
					"VALUES ((SELECT min(id) FROM cruises), 'Hidden', $1, $2)", years[3], dates[3])
				execSeed(t, conn, "INSERT INTO cruises (name) VALUES ('Hidden parent')")
				execSeed(t, conn, "INSERT INTO voyages (cruise_id, name, year, embark_date, published_at) "+
					"VALUES ((SELECT max(id) FROM cruises), 'In hidden', $1, $2, now())", years[3], dates[3])
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
	for _, tc := range keysetCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.callerFor(user.ID)
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
	for _, tc := range keysetCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.callerFor(user.ID)
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

// TestListPaging_CruisesNewestFirst checks the cruise list's order: by id
// descending, newest cruise first, in a cursor walk as in one page. A member
// skips the hidden draft between two pages; an admin sees it in place.
func TestListPaging_CruisesNewestFirst(t *testing.T) {
	conn := lockTestDB(t)
	q := sqlcdb.New(conn)
	user, err := q.CreateUser(context.Background(), sqlcdb.CreateUserParams{Email: "pager@example.com"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	execSeed(t, conn, "TRUNCATE cruises RESTART IDENTITY CASCADE")
	insert := func(stmt string) int64 {
		t.Helper()
		var id int64
		if err := conn.QueryRowContext(context.Background(), stmt).Scan(&id); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
		return id
	}
	const published = "INSERT INTO cruises (name, published_at) VALUES ('C', now()) RETURNING id"
	first, second, third := insert(published), insert(published), insert(published)
	hidden := insert("INSERT INTO cruises (name) VALUES ('Hidden') RETURNING id")
	newest := insert(published)
	_, api := humatest.New(t)
	RegisterCruiseRoutes(api, q, conn)

	walk := func(ctx context.Context) []int64 {
		t.Helper()
		var ids []int64
		path := "/cruises?limit=2"
		for requests := 1; ; requests++ {
			if requests > 10 {
				t.Fatalf("walk did not end; walked %v", ids)
			}
			p := getIDPage(t, api, ctx, path)
			ids = append(ids, pageIDs(p)...)
			if !p.HasMore {
				return ids
			}
			path = "/cruises?limit=2&after=" + url.QueryEscape(p.NextCursor)
		}
	}
	tests := []struct {
		name string
		ctx  context.Context
		want []int64
	}{
		{"member", memberCtx(user.ID), []int64{newest, third, second, first}},
		{"admin", callerCtx(user.ID, "admin"), []int64{newest, hidden, third, second, first}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := walk(tt.ctx); !slices.Equal(got, tt.want) {
				t.Errorf("cursor walk = %v, want %v", got, tt.want)
			}
			if got := pageIDs(getIDPage(t, api, tt.ctx, "/cruises?limit=100")); !slices.Equal(got, tt.want) {
				t.Errorf("single page = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestListPaging_LongNameCursor follows a cursor past a person whose name is
// as long as onboarding allows (two 100-letter Polish names), which makes
// next_cursor far longer than a short name would.
func TestListPaging_LongNameCursor(t *testing.T) {
	conn := lockTestDB(t)
	q := sqlcdb.New(conn)
	long := "A" + strings.Repeat("ż", 99) + " " + strings.Repeat("ż", 100)
	for _, n := range []string{long, "B"} {
		execSeed(t, conn, "INSERT INTO crew_members (full_name) VALUES ($1)", n)
	}
	_, api := humatest.New(t)
	RegisterCrewRoutes(api, q, conn, nil)
	ctx := userCtx(context.Background())

	first := getIDPage(t, api, ctx, "/crew?limit=1")
	if len(first.NextCursor) <= 512 {
		t.Fatalf("cursor length %d, want a long one past 512", len(first.NextCursor))
	}
	second := getIDPage(t, api, ctx, "/crew?limit=1&after="+url.QueryEscape(first.NextCursor))
	if len(second.Items) != 1 || second.HasMore {
		t.Fatalf("second page = %+v, want the one remaining person", second)
	}
}
