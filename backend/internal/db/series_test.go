package db

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func text(s string) types.NullString { return types.NullString{String: s, Valid: true} }

// TestCruiseSeries_Editions checks the editions query against a real
// database: newest edition first, totals summed over each edition's voyages,
// and editions without voyages reported as zero.
func TestCruiseSeries_Editions(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	q := sqlcdb.New(conn)

	series, err := q.CreateCruiseSeries(ctx, sqlcdb.CreateCruiseSeriesParams{Name: "May I Sail"})
	if err != nil {
		t.Fatalf("create series: %v", err)
	}
	other, err := q.CreateCruiseSeries(ctx, sqlcdb.CreateCruiseSeriesParams{Name: "Other"})
	if err != nil {
		t.Fatalf("create other series: %v", err)
	}
	inSeries := types.NullInt64{Int64: series.ID, Valid: true}

	newCruise := func(name, embark, edition string, seriesID types.NullInt64) sqlcdb.Cruise {
		t.Helper()
		c, err := q.CreateCruise(ctx, sqlcdb.CreateCruiseParams{
			Name: name, EmbarkDate: text(embark), Kind: "sea", SeriesID: seriesID, Edition: text(edition),
		})
		if err != nil {
			t.Fatalf("create cruise %s: %v", name, err)
		}
		return c
	}
	older := newCruise("MIS 2024", "2024-05-01", "2024", inSeries)
	newer := newCruise("MIS 2025", "2025-05-01", "2025", inSeries)
	empty := newCruise("MIS 2026", "2026-05-01", "2026", inSeries)
	unrelated := newCruise("Other 2025", "2025-06-01", "2025", types.NullInt64{Int64: other.ID, Valid: true})

	addVoyage := func(cruiseID int64, miles, hours float64) {
		t.Helper()
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO voyages (cruise_id, name, miles, hours_total) VALUES ($1, 'leg', $2, $3)",
			cruiseID, miles, hours); err != nil {
			t.Fatalf("insert voyage: %v", err)
		}
	}
	addVoyage(older.ID, 100, 20)
	addVoyage(newer.ID, 150.5, 30)
	addVoyage(newer.ID, 49.5, 10)
	addVoyage(unrelated.ID, 999, 999)

	rows, err := q.ListCruiseSeriesEditions(ctx, inSeries)
	if err != nil {
		t.Fatalf("list editions: %v", err)
	}
	type got struct {
		id           int64
		voyages      int64
		miles, hours float64
	}
	want := []got{
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

// TestCruiseSeries_DeleteKeepsCruises checks that deleting a series keeps
// its cruises and clears their link, and that an unknown series is rejected
// by the named foreign key the API maps to 422.
func TestCruiseSeries_DeleteKeepsCruises(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	q := sqlcdb.New(conn)

	series, err := q.CreateCruiseSeries(ctx, sqlcdb.CreateCruiseSeriesParams{Name: "Navigare Necesse Est"})
	if err != nil {
		t.Fatalf("create series: %v", err)
	}
	cruise, err := q.CreateCruise(ctx, sqlcdb.CreateCruiseParams{
		Name: "NNE 2025", Kind: "sea", SeriesID: types.NullInt64{Int64: series.ID, Valid: true}, Edition: text("2025"),
	})
	if err != nil {
		t.Fatalf("create cruise: %v", err)
	}
	if err := q.DeleteCruiseSeries(ctx, series.ID); err != nil {
		t.Fatalf("delete series: %v", err)
	}
	after, err := q.GetCruise(ctx, cruise.ID)
	if err != nil {
		t.Fatalf("cruise must survive series delete: %v", err)
	}
	if after.SeriesID.Valid {
		t.Errorf("series_id = %v, want null", after.SeriesID)
	}

	_, err = q.CreateCruise(ctx, sqlcdb.CreateCruiseParams{
		Name: "Ghost", Kind: "sea", SeriesID: types.NullInt64{Int64: series.ID, Valid: true},
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" || pgErr.ConstraintName != "cruises_series_id_fkey" {
		t.Fatalf("create with deleted series: err = %v, want FK violation on cruises_series_id_fkey", err)
	}
}
