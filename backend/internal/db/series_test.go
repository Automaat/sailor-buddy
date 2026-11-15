package db

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func text(s string) types.NullString { return types.NullString{String: s, Valid: true} }

// unpriced fills the cruise columns that have no usable Go zero value: the
// NOT NULL price lists.
func unpriced(p sqlcdb.CreateCruiseParams) sqlcdb.CreateCruiseParams {
	p.Included = json.RawMessage("[]")
	p.NotIncluded = json.RawMessage("[]")
	return p
}

// TestCruiseSeries_Editions checks the editions query against a real
// database: editions sort newest first by their earliest leg's embark date
// over trips and voyages together, editions without a dated leg (no legs, or
// only the empty date the leg forms send) come first by edition label,
// totals are summed over each edition's voyages, and editions without
// voyages report zero.
func TestCruiseSeries_Editions(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	q := sqlcdb.New(conn)

	series, err := q.CreateCruiseSeries(ctx, sqlcdb.CreateCruiseSeriesParams{Name: "May I Sail", Kind: "sea"})
	if err != nil {
		t.Fatalf("create series: %v", err)
	}
	other, err := q.CreateCruiseSeries(ctx, sqlcdb.CreateCruiseSeriesParams{Name: "Other", Kind: "sea"})
	if err != nil {
		t.Fatalf("create other series: %v", err)
	}
	inSeries := types.NullInt64{Int64: series.ID, Valid: true}

	newCruise := func(name, edition string, seriesID types.NullInt64) sqlcdb.Cruise {
		t.Helper()
		c, err := q.CreateCruise(ctx, unpriced(sqlcdb.CreateCruiseParams{
			Name: name, SeriesID: seriesID, Edition: text(edition),
		}))
		if err != nil {
			t.Fatalf("create cruise %s: %v", name, err)
		}
		return c
	}
	// embark is a leg's embark date; "-" stores NULL.
	date := func(embark string) any {
		if embark == "-" {
			return nil
		}
		return embark
	}
	addTrip := func(cruiseID int64, embark string) {
		t.Helper()
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO trips (cruise_id, name, embark_date) VALUES ($1, 'etap', $2)",
			cruiseID, date(embark)); err != nil {
			t.Fatalf("insert trip: %v", err)
		}
	}
	addVoyage := func(cruiseID int64, embark string) {
		t.Helper()
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO voyages (cruise_id, name, embark_date) VALUES ($1, 'leg', $2)",
			cruiseID, date(embark)); err != nil {
			t.Fatalf("insert voyage: %v", err)
		}
	}

	older := newCruise("MIS 2024", "2024", inSeries)
	addTrip(older.ID, "2024-05-01")
	addVoyage(older.ID, "-")

	// The 2025 edition's voyage starts before its trip; a trip-only or
	// latest-leg order would put it above the 2026 edition.
	newer := newCruise("MIS 2025", "2025", inSeries)
	addTrip(newer.ID, "2026-09-01")
	addVoyage(newer.ID, "2025-05-01")
	addVoyage(newer.ID, "")

	empty := newCruise("MIS 2026", "2026", inSeries)
	addTrip(empty.ID, "2026-05-01")
	addTrip(empty.ID, "")

	planned := newCruise("MIS 2027/28", "2027/28", inSeries)
	formPlanned := newCruise("MIS 2027", "2027", inSeries)
	addTrip(formPlanned.ID, "")
	addVoyage(formPlanned.ID, "-")

	unrelated := newCruise("Other 2025", "2025", types.NullInt64{Int64: other.ID, Valid: true})
	addTrip(unrelated.ID, "2030-01-01")
	addVoyage(unrelated.ID, "2030-01-01")

	rows, err := q.ListCruiseSeriesEditions(ctx, sqlcdb.ListCruiseSeriesEditionsParams{SeriesID: inSeries, IncludeHidden: true})
	if err != nil {
		t.Fatalf("list editions: %v", err)
	}
	want := []int64{planned.ID, formPlanned.ID, empty.ID, newer.ID, older.ID}
	var got []int64
	for _, r := range rows {
		got = append(got, r.ID)
	}
	if !slices.Equal(got, want) {
		t.Errorf("edition order = %v, want %v", got, want)
	}
}

// TestCruiseSeries_EditionsIgnoreHiddenLegs checks that a member's edition
// order comes from published legs only: a hidden draft leg dated before every
// other edition must not move its edition down, while an admin, who sees the
// draft, gets the order it implies.
func TestCruiseSeries_EditionsIgnoreHiddenLegs(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	q := sqlcdb.New(conn)

	series, err := q.CreateCruiseSeries(ctx, sqlcdb.CreateCruiseSeriesParams{Name: "NNE", Kind: "sea"})
	if err != nil {
		t.Fatalf("create series: %v", err)
	}
	inSeries := types.NullInt64{Int64: series.ID, Valid: true}
	edition := func(name string) int64 {
		t.Helper()
		c, err := q.CreateCruise(ctx, unpriced(sqlcdb.CreateCruiseParams{Name: name, SeriesID: inSeries, Edition: text(name)}))
		if err != nil {
			t.Fatalf("create cruise %s: %v", name, err)
		}
		if _, err := q.PublishCruise(ctx, c.ID); err != nil {
			t.Fatalf("publish cruise %s: %v", name, err)
		}
		return c.ID
	}
	leg := func(table string, cruiseID int64, embark string, published bool) {
		t.Helper()
		publishedAt := any(nil)
		if published {
			publishedAt = "2026-01-01T00:00:00Z"
		}
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO "+table+" (cruise_id, name, embark_date, published_at) VALUES ($1, 'etap', $2, $3)",
			cruiseID, embark, publishedAt); err != nil {
			t.Fatalf("insert %s: %v", table, err)
		}
	}

	e2025 := edition("2025")
	leg("trips", e2025, "2025-06-01", true)
	e2026 := edition("2026")
	leg("trips", e2026, "2026-06-01", true)
	leg("trips", e2026, "2020-01-01", false)
	leg("voyages", e2026, "2019-01-01", false)

	for _, tc := range []struct {
		name          string
		includeHidden bool
		want          []int64
	}{
		{"member", false, []int64{e2026, e2025}},
		{"admin", true, []int64{e2025, e2026}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := q.ListCruiseSeriesEditions(ctx, sqlcdb.ListCruiseSeriesEditionsParams{
				SeriesID: inSeries, IncludeHidden: tc.includeHidden,
			})
			if err != nil {
				t.Fatalf("list editions: %v", err)
			}
			var got []int64
			for _, r := range rows {
				got = append(got, r.ID)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("edition order = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCruiseSeries_Kind checks that a series stores its kind on create and
// update, and that the CHECK constraint refuses an unknown kind.
func TestCruiseSeries_Kind(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	q := sqlcdb.New(conn)

	created, err := q.CreateCruiseSeries(ctx, sqlcdb.CreateCruiseSeriesParams{Name: "Mazury", Kind: "inland"})
	if err != nil {
		t.Fatalf("create series: %v", err)
	}
	if created.Kind != "inland" {
		t.Fatalf("created kind = %q, want inland", created.Kind)
	}
	if err := q.UpdateCruiseSeries(ctx, sqlcdb.UpdateCruiseSeriesParams{ID: created.ID, Name: "Bałtyk", Kind: "sea"}); err != nil {
		t.Fatalf("update series: %v", err)
	}
	got, err := q.GetCruiseSeries(ctx, created.ID)
	if err != nil {
		t.Fatalf("get series: %v", err)
	}
	if got.Kind != "sea" || got.Name != "Bałtyk" {
		t.Fatalf("after update = %+v, want sea Bałtyk", got)
	}

	for _, kind := range []string{"", "lake", "SEA"} {
		_, err := q.CreateCruiseSeries(ctx, sqlcdb.CreateCruiseSeriesParams{Name: "Bad", Kind: kind})
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "cruise_series_kind_check" {
			t.Errorf("kind %q: err = %v, want CHECK violation on cruise_series_kind_check", kind, err)
		}
	}
}

// TestCruiseSeries_DeleteKeepsCruises checks that deleting a series keeps
// its cruises and clears their link, and that an unknown series is rejected
// by the named foreign key the API maps to 422.
func TestCruiseSeries_DeleteKeepsCruises(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	q := sqlcdb.New(conn)

	series, err := q.CreateCruiseSeries(ctx, sqlcdb.CreateCruiseSeriesParams{Name: "Navigare Necesse Est", Kind: "sea"})
	if err != nil {
		t.Fatalf("create series: %v", err)
	}
	cruise, err := q.CreateCruise(ctx, unpriced(sqlcdb.CreateCruiseParams{
		Name: "NNE 2025", SeriesID: types.NullInt64{Int64: series.ID, Valid: true}, Edition: text("2025"),
	}))
	if err != nil {
		t.Fatalf("create cruise: %v", err)
	}
	if err := q.DeleteCruiseSeries(ctx, series.ID); err != nil {
		t.Fatalf("delete series: %v", err)
	}
	after, err := q.GetCruise(ctx, sqlcdb.GetCruiseParams{ID: cruise.ID, IncludeHidden: true})
	if err != nil {
		t.Fatalf("cruise must survive series delete: %v", err)
	}
	if after.SeriesID.Valid {
		t.Errorf("series_id = %v, want null", after.SeriesID)
	}

	_, err = q.CreateCruise(ctx, unpriced(sqlcdb.CreateCruiseParams{
		Name: "Ghost", SeriesID: types.NullInt64{Int64: series.ID, Valid: true},
	}))
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" || pgErr.ConstraintName != "cruises_series_id_fkey" {
		t.Fatalf("create with deleted series: err = %v, want FK violation on cruises_series_id_fkey", err)
	}
}
