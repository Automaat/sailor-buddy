package db

import (
	"context"
	"encoding/json"
	"errors"
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
	addVoyage := func(cruiseID int64, embark string, miles, hours float64) {
		t.Helper()
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO voyages (cruise_id, name, embark_date, miles, hours_total) VALUES ($1, 'leg', $2, $3, $4)",
			cruiseID, date(embark), miles, hours); err != nil {
			t.Fatalf("insert voyage: %v", err)
		}
	}

	older := newCruise("MIS 2024", "2024", inSeries)
	addTrip(older.ID, "2024-05-01")
	addVoyage(older.ID, "-", 100, 20)

	// The 2025 edition's voyage starts before its trip; a trip-only or
	// latest-leg order would put it above the 2026 edition.
	newer := newCruise("MIS 2025", "2025", inSeries)
	addTrip(newer.ID, "2026-09-01")
	addVoyage(newer.ID, "2025-05-01", 150.5, 30)
	addVoyage(newer.ID, "", 49.5, 10)

	empty := newCruise("MIS 2026", "2026", inSeries)
	addTrip(empty.ID, "2026-05-01")
	addTrip(empty.ID, "")

	planned := newCruise("MIS 2027/28", "2027/28", inSeries)
	formPlanned := newCruise("MIS 2027", "2027", inSeries)
	addTrip(formPlanned.ID, "")
	addVoyage(formPlanned.ID, "-", 0, 0)

	unrelated := newCruise("Other 2025", "2025", types.NullInt64{Int64: other.ID, Valid: true})
	addTrip(unrelated.ID, "2030-01-01")
	addVoyage(unrelated.ID, "2030-01-01", 999, 999)

	rows, err := q.ListCruiseSeriesEditions(ctx, sqlcdb.ListCruiseSeriesEditionsParams{SeriesID: inSeries, IncludeHidden: true})
	if err != nil {
		t.Fatalf("list editions: %v", err)
	}
	type got struct {
		id           int64
		voyages      int64
		miles, hours float64
	}
	want := []got{
		{planned.ID, 0, 0, 0},
		{formPlanned.ID, 1, 0, 0},
		{empty.ID, 0, 0, 0},
		{newer.ID, 2, 200, 40},
		{older.ID, 1, 100, 20},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d editions, want %d: %+v", len(rows), len(want), rows)
	}
	for i, w := range want {
		r := rows[i]
		if (got{r.ID, r.VoyageCount, r.TotalMiles, r.TotalHours}) != w {
			t.Errorf("edition %d = %+v, want %+v", i, r, w)
		}
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
