package handlers

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// errTripCruiseChanged reports that the trip moved to another cruise between
// the cruise lock and the trip lock on every completion attempt.
var errTripCruiseChanged = errors.New("trip cruise changed during completion")

// errTripCaptainChanged reports that the trip's captain person changed
// between the captain lock and the trip lock on every completion attempt.
var errTripCaptainChanged = errors.New("trip captain changed during completion")

// errTripYachtChanged reports that the trip's yacht changed between the yacht
// lock and the trip lock on every completion attempt.
var errTripYachtChanged = errors.New("trip yacht changed during completion")

// completeTripAttempts bounds the retries when the trip's cruise, captain or
// yacht changes between its lock and the trip lock.
const completeTripAttempts = 3

// completeTripTx wraps the trip → voyage transition: it creates the voyage,
// copies ports, repoints crew assignments, drops enrollments and deletes the
// trip — all in one transaction.
func completeTripTx(ctx context.Context, db *sql.DB, tripID int64, req dto.CompleteTripBody) (sqlcdb.Voyage, error) {
	return completeTripTxWith(ctx, db, nil, tripID, req)
}

// completeTripTxWith runs completeTripTx with an optional querier wrapper,
// which tests use to pause the transaction between statements.
func completeTripTxWith(ctx context.Context, db *sql.DB, wrap func(sqlcdb.Querier) sqlcdb.Querier, tripID int64, req dto.CompleteTripBody) (sqlcdb.Voyage, error) {
	var last error
	for range completeTripAttempts {
		voyage, err := completeTripOnce(ctx, db, wrap, tripID, req)
		if !errors.Is(err, errTripCruiseChanged) && !errors.Is(err, errTripCaptainChanged) &&
			!errors.Is(err, errTripYachtChanged) {
			return voyage, err
		}
		last = err
	}
	return sqlcdb.Voyage{}, last
}

func completeTripOnce(ctx context.Context, db *sql.DB, wrap func(sqlcdb.Querier) sqlcdb.Querier, tripID int64, req dto.CompleteTripBody) (sqlcdb.Voyage, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return sqlcdb.Voyage{}, &QueryError{Op: "BeginTx", Err: err}
	}
	defer func() { _ = tx.Rollback() }()

	var qtx sqlcdb.Querier = sqlcdb.New(tx)
	if wrap != nil {
		qtx = wrap(qtx)
	}

	// Lock the cruise, the captain person and the yacht, then the trip,
	// before any write. DELETE FROM cruises takes the same order (cruise,
	// then the trips foreign-key check), so does deleting the captain person
	// (person, then its ON DELETE SET NULL on trips) and deleting the yacht
	// (yacht, then the trips foreign-key check), and a concurrent leg
	// enrollment share-locks the trip; any other order deadlocks with one of
	// them.
	cruiseID, err := qtx.LockTripCruise(ctx, tripID)
	locked := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return sqlcdb.Voyage{}, &QueryError{Op: "LockTripCruise", Err: err}
	}
	captainID, err := qtx.LockTripCaptain(ctx, tripID)
	captain := types.NullInt64{Int64: captainID, Valid: err == nil}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return sqlcdb.Voyage{}, &QueryError{Op: "LockTripCaptain", Err: err}
	}
	yachtID, err := qtx.LockTripYacht(ctx, tripID)
	yacht := types.NullInt64{Int64: yachtID, Valid: err == nil}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return sqlcdb.Voyage{}, &QueryError{Op: "LockTripYacht", Err: err}
	}
	trip, err := qtx.GetTripForUpdate(ctx, tripID)
	if err != nil {
		return sqlcdb.Voyage{}, err
	}
	// A trip that moved to a cruise, captain or yacht other than the locked
	// one must retry, since locking the new row now would invert the order.
	if !locked || trip.CruiseID != cruiseID {
		return sqlcdb.Voyage{}, errTripCruiseChanged
	}
	if trip.CaptainPersonID != captain {
		return sqlcdb.Voyage{}, errTripCaptainChanged
	}
	if trip.YachtID != yacht {
		return sqlcdb.Voyage{}, errTripYachtChanged
	}
	if trip.Status != sqlcdb.TripStatusPlanned {
		return sqlcdb.Voyage{}, sql.ErrNoRows
	}

	voyage, err := createVoyageFromTrip(ctx, qtx, trip, req)
	if err != nil {
		return sqlcdb.Voyage{}, &QueryError{Op: "CreateVoyageFromTrip", Err: err}
	}

	for i, port := range req.Ports {
		position := int64(i)
		if port.Position != nil {
			position = *port.Position
		}
		if _, err := qtx.CreateVoyagePort(ctx, sqlcdb.CreateVoyagePortParams{
			VoyageID:  voyage.ID,
			Name:      port.Name,
			Latitude:  nullFloat64(port.Latitude),
			Longitude: nullFloat64(port.Longitude),
			Anchorage: port.Anchorage,
			Position:  position,
		}); err != nil {
			return sqlcdb.Voyage{}, &QueryError{Op: "CreateVoyagePort", Err: err}
		}
	}

	if err := qtx.RepointCrewAssignmentsToVoyage(ctx, sqlcdb.RepointCrewAssignmentsToVoyageParams{
		VoyageID: types.NullInt64{Int64: voyage.ID, Valid: true},
		TripID:   types.NullInt64{Int64: trip.ID, Valid: true},
	}); err != nil {
		return sqlcdb.Voyage{}, &QueryError{Op: "RepointCrewAssignmentsToVoyage", Err: err}
	}

	if err := qtx.DeleteTripEnrollmentsForTrip(ctx, trip.ID); err != nil {
		return sqlcdb.Voyage{}, &QueryError{Op: "DeleteTripEnrollmentsForTrip", Err: err}
	}

	if err := qtx.DeleteTrip(ctx, trip.ID); err != nil {
		return sqlcdb.Voyage{}, &QueryError{Op: "DeleteTrip", Err: err}
	}

	if err := tx.Commit(); err != nil {
		return sqlcdb.Voyage{}, &QueryError{Op: "Commit", Err: err}
	}
	return voyage, nil
}

// createVoyageFromTrip inserts the voyage row for a completed trip. The voyage
// year falls back to the trip embark date when the request omits it.
func createVoyageFromTrip(ctx context.Context, qtx sqlcdb.Querier, trip sqlcdb.Trip, req dto.CompleteTripBody) (sqlcdb.Voyage, error) {
	year := req.Year
	if year == nil && trip.EmbarkDate.Valid {
		if t, perr := time.Parse(time.DateOnly, trip.EmbarkDate.String); perr == nil {
			y := int64(t.Year())
			year = &y
		}
	}

	return qtx.CreateVoyage(ctx, sqlcdb.CreateVoyageParams{
		CreatedBy:       trip.CreatedBy,
		CruiseID:        trip.CruiseID,
		Name:            trip.Name,
		Year:            nullInt64(year),
		EmbarkDate:      trip.EmbarkDate,
		DisembarkDate:   trip.DisembarkDate,
		Countries:       trip.Countries,
		StartPort:       trip.StartPort,
		EndPort:         trip.EndPort,
		CaptainName:     trip.CaptainName,
		CaptainPersonID: trip.CaptainPersonID,
		YachtID:         trip.YachtID,
		HoursTotal:      valOrZeroFloat(req.HoursTotal),
		HoursSail:       valOrZeroFloat(req.HoursSail),
		HoursEngine:     valOrZeroFloat(req.HoursEngine),
		HoursOver6bf:    valOrZeroFloat(req.HoursOver6bf),
		Miles:           valOrZeroFloat(req.Miles),
		Days:            valOrZeroInt(req.Days),
		TidalWaters:     valOrZeroInt(req.TidalWaters),
		CostTotal:       trip.CostTotal,
		CostPerPerson:   trip.CostPerPerson,
		ImageLogoUrl:    trip.ImageLogoUrl,
		ImagePhotoUrl:   trip.ImagePhotoUrl,
		ImageRouteUrl:   trip.ImageRouteUrl,
		Description:     trip.Description,
		PublishedAt:     trip.PublishedAt,
	})
}
