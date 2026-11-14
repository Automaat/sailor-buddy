package db

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

var (
	queryNameRe = regexp.MustCompile(`(?m)^-- name: (\w+)`)
	orderByRe   = regexp.MustCompile(`(?is)ORDER BY\s+(.+?)\s+LIMIT\b`)
	sortTailRe  = regexp.MustCompile(`(?i)\s+(ASC|DESC|NULLS\s+(FIRST|LAST))\b`)
	offsetRe    = regexp.MustCompile(`(?i)\bOFFSET\b`)
)

// pagedQueries maps each sqlc query that pages with OFFSET to the last
// term of its ORDER BY clause, with direction and NULLS placement removed.
func pagedQueries(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("queries", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("glob queries: %v (%d files)", err, len(files))
	}
	out := map[string]string{}
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		text := string(src)
		starts := queryNameRe.FindAllStringSubmatchIndex(text, -1)
		for i, loc := range starts {
			end := len(text)
			if i+1 < len(starts) {
				end = starts[i+1][0]
			}
			body := text[loc[0]:end]
			if !offsetRe.MatchString(body) {
				continue
			}
			name := text[loc[2]:loc[3]]
			m := orderByRe.FindStringSubmatch(body)
			if m == nil {
				out[name] = ""
				continue
			}
			terms := strings.Split(m[1], ",")
			out[name] = strings.TrimSpace(sortTailRe.ReplaceAllString(terms[len(terms)-1], ""))
		}
	}
	return out
}

// TestPagedQueries_OrderEndsInID guards the offset-paged list queries the
// frontend walks page by page: without a unique last sort key, rows that tie
// on the leading keys may swap between page requests and repeat or vanish.
func TestPagedQueries_OrderEndsInID(t *testing.T) {
	got := pagedQueries(t)
	for _, want := range []string{
		"ListCrewMembers", "ListCruiseSeries", "ListCruises", "ListTrainings",
		"ListTrips", "ListVoyages", "ListYachts",
	} {
		if _, ok := got[want]; !ok {
			t.Errorf("%s is not detected as an OFFSET-paged query", want)
		}
	}
	for name, last := range got {
		if last != "id" && !strings.HasSuffix(last, ".id") {
			t.Errorf("%s: ORDER BY must end in the unique id column, ends in %q", name, last)
		}
	}
}

// TestListCrewMembers_PagesTiedNamesOnce pages through people who share one
// full name with a page size smaller than the tie and checks every person
// comes back exactly once, in id order.
func TestListCrewMembers_PagesTiedNamesOnce(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	q := sqlcdb.New(conn)

	var want []int64
	for i := range 7 {
		name := "Jan Kowalski"
		if i == 3 {
			name = "Adam Nowak"
		}
		p, err := q.CreateCrewMember(ctx, sqlcdb.CreateCrewMemberParams{
			FullName: name, OtherNames: json.RawMessage("[]"),
		})
		if err != nil {
			t.Fatalf("create person %d: %v", i, err)
		}
		if name == "Jan Kowalski" {
			want = append(want, p.ID)
		}
	}

	var got []int64
	for offset := int32(0); ; offset += 2 {
		rows, err := q.ListCrewMembers(ctx, sqlcdb.ListCrewMembersParams{RowLimit: 2, RowOffset: offset})
		if err != nil {
			t.Fatalf("list offset %d: %v", offset, err)
		}
		if len(rows) == 0 {
			break
		}
		for _, r := range rows {
			if r.FullName == "Jan Kowalski" {
				got = append(got, r.ID)
			}
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("tied names paged as %v, want each once in id order %v", got, want)
	}
}
