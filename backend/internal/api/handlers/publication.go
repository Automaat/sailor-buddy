package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// PublicationHandler publishes and hides cruises, trips and voyages. A
// record with no published_at is a hidden draft: only admins and the captain
// group (canSeeHidden) read it. A trip or voyage is visible only while its
// cruise is published too.
type PublicationHandler struct {
	q     sqlcdb.Querier
	runTx txRunner
}

// errLegCruiseChanged reports that the trip or voyage being published moved
// to another cruise between the cruise lock and the leg update on every
// publish attempt.
var errLegCruiseChanged = errors.New("leg cruise changed during publish")

// publishLegAttempts bounds the leg publish transactions retryLegMove runs.
const publishLegAttempts = 3

// retryLegMove runs a leg publish transaction again while it fails with
// errLegCruiseChanged, up to publishLegAttempts times in all. Locking the
// leg's new cruise while holding the leg would invert the cruise-then-leg
// order publishCruise uses, so the attempt rolls back and the next one locks
// the new cruise first.
func retryLegMove(ctx context.Context, run txRunner, fn func(sqlcdb.Querier) error) error {
	var err error
	for range publishLegAttempts {
		if err = run(ctx, fn); !errors.Is(err, errLegCruiseChanged) {
			return err
		}
	}
	return err
}

type publishCruiseInput struct {
	ID   int64                  `path:"cruiseID" doc:"Cruise ID"`
	Body *dto.PublishCruiseBody `required:"false"`
}

// RegisterPublicationRoutes wires the admin-only publish and unpublish
// operations of cruises, trips and voyages onto the API.
func RegisterPublicationRoutes(api huma.API, q sqlcdb.Querier, db *sql.DB) {
	h := &PublicationHandler{q: q, runTx: retryDeadlock(sqlTxRunner(db))}

	huma.Register(api, huma.Operation{
		OperationID: "publish-cruise", Method: http.MethodPost, Path: "/cruises/{cruiseID}/publish",
		Summary: "Publish a cruise (admin)",
		Description: "Makes the cruise visible to every member and keeps an earlier publication time. " +
			"With include_legs, its hidden trips and voyages are published in the same transaction.",
		Tags: []string{"Cruises"},
	}, h.publishCruise)
	huma.Register(api, huma.Operation{
		OperationID: "unpublish-cruise", Method: http.MethodPost, Path: "/cruises/{cruiseID}/unpublish",
		Summary: "Hide a cruise (admin)",
		Description: "Hides the cruise, and with it all of its trips and voyages, from members " +
			"outside the captain group. Allowed while it has enrollments.",
		Tags: []string{"Cruises"},
	}, h.unpublishCruise)
	huma.Register(api, huma.Operation{
		OperationID: "publish-trip", Method: http.MethodPost, Path: "/trips/{tripID}/publish",
		Summary: "Publish a trip (admin)",
		Description: "Keeps an earlier publication time. A hidden cruise without a series is " +
			"published with it; a trip in a hidden series edition stays hidden until the edition is published.",
		Tags: []string{"Trips"},
	}, h.publishTrip)
	huma.Register(api, huma.Operation{
		OperationID: "unpublish-trip", Method: http.MethodPost, Path: "/trips/{tripID}/unpublish",
		Summary: "Hide a trip (admin)", Description: "Allowed while it has enrollments.",
		Tags: []string{"Trips"},
	}, h.unpublishTrip)
	huma.Register(api, huma.Operation{
		OperationID: "publish-voyage", Method: http.MethodPost, Path: "/voyages/{voyageID}/publish",
		Summary: "Publish a voyage (admin)",
		Description: "Keeps an earlier publication time. A hidden cruise without a series is " +
			"published with it; a voyage in a hidden series edition stays hidden until the edition is published.",
		Tags: []string{"Voyages"},
	}, h.publishVoyage)
	huma.Register(api, huma.Operation{
		OperationID: "unpublish-voyage", Method: http.MethodPost, Path: "/voyages/{voyageID}/unpublish",
		Summary: "Hide a voyage (admin)",
		Tags:    []string{"Voyages"},
	}, h.unpublishVoyage)
}

// publishCruise makes the cruise visible. With include_legs it also publishes
// the cruise's hidden trips and voyages, cruise first: the lock order of a
// share-link sign-up (cruise, then its trips).
func (h *PublicationHandler) publishCruise(ctx context.Context, in *publishCruiseInput) (*cruiseOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	includeLegs := in.Body != nil && in.Body.IncludeLegs
	var cruise sqlcdb.Cruise
	err := h.runTx(ctx, func(q sqlcdb.Querier) error {
		var err error
		if cruise, err = q.PublishCruise(ctx, in.ID); err != nil {
			return err
		}
		if !includeLegs {
			return nil
		}
		if err := q.PublishCruiseTrips(ctx, in.ID); err != nil {
			return &QueryError{Op: "PublishCruiseTrips", Err: err}
		}
		if err := q.PublishCruiseVoyages(ctx, in.ID); err != nil {
			return &QueryError{Op: "PublishCruiseVoyages", Err: err}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("cruise not found")
		}
		slog.Error("publish cruise", "cruise_id", in.ID, "include_legs", includeLegs, "err", err)
		return nil, huma.Error500InternalServerError("failed to publish cruise")
	}
	return &cruiseOutput{Body: dto.CruiseFromDB(cruise)}, nil
}

// unpublishCruise hides the cruise. Its legs keep their own flag but are
// hidden with it.
func (h *PublicationHandler) unpublishCruise(ctx context.Context, in *cruiseIDParam) (*cruiseOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	cruise, err := h.q.UnpublishCruise(ctx, in.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("cruise not found")
		}
		slog.Error("unpublish cruise", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to unpublish cruise")
	}
	return &cruiseOutput{Body: dto.CruiseFromDB(cruise)}, nil
}

// publishTrip publishes the trip and, when its cruise is a hidden standalone
// wrapper (no series), that cruise too, in one transaction. It locks the
// cruise before the trip, the order publishCruise uses, and starts over when
// the trip moved to another cruise in between.
func (h *PublicationHandler) publishTrip(ctx context.Context, in *tripIDParam) (*tripOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	var trip sqlcdb.Trip
	err := retryLegMove(ctx, h.runTx, func(q sqlcdb.Querier) error {
		cruiseID, err := q.LockTripCruiseForPublish(ctx, in.ID)
		if err != nil {
			return err
		}
		if trip, err = q.PublishTrip(ctx, in.ID); err != nil {
			return err
		}
		if trip.CruiseID != cruiseID {
			return errLegCruiseChanged
		}
		if err := q.PublishStandaloneCruise(ctx, cruiseID); err != nil {
			return &QueryError{Op: "PublishStandaloneCruise", Err: err}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found")
		}
		if errors.Is(err, errLegCruiseChanged) {
			return nil, huma.Error409Conflict("trip moved to another cruise; retry")
		}
		slog.Error("publish trip", "trip_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to publish trip")
	}
	return &tripOutput{Body: dto.TripFromDB(trip)}, nil
}

func (h *PublicationHandler) unpublishTrip(ctx context.Context, in *tripIDParam) (*tripOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	trip, err := h.q.UnpublishTrip(ctx, in.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found")
		}
		slog.Error("unpublish trip", "trip_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to unpublish trip")
	}
	return &tripOutput{Body: dto.TripFromDB(trip)}, nil
}

// publishVoyage publishes the voyage and, when its cruise is a hidden
// standalone wrapper (no series), that cruise too, in one transaction. It
// locks the cruise before the voyage, the order publishCruise uses, and
// starts over when the voyage moved to another cruise in between.
func (h *PublicationHandler) publishVoyage(ctx context.Context, in *voyageIDParam) (*voyageOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	var voyage sqlcdb.Voyage
	err := retryLegMove(ctx, h.runTx, func(q sqlcdb.Querier) error {
		cruiseID, err := q.LockVoyageCruiseForPublish(ctx, in.ID)
		if err != nil {
			return err
		}
		if voyage, err = q.PublishVoyage(ctx, in.ID); err != nil {
			return err
		}
		if voyage.CruiseID != cruiseID {
			return errLegCruiseChanged
		}
		if err := q.PublishStandaloneCruise(ctx, cruiseID); err != nil {
			return &QueryError{Op: "PublishStandaloneCruise", Err: err}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("voyage not found")
		}
		if errors.Is(err, errLegCruiseChanged) {
			return nil, huma.Error409Conflict("voyage moved to another cruise; retry")
		}
		slog.Error("publish voyage", "voyage_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to publish voyage")
	}
	return &voyageOutput{Body: dto.VoyageFromDB(voyage)}, nil
}

func (h *PublicationHandler) unpublishVoyage(ctx context.Context, in *voyageIDParam) (*voyageOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	voyage, err := h.q.UnpublishVoyage(ctx, in.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("voyage not found")
		}
		slog.Error("unpublish voyage", "voyage_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to unpublish voyage")
	}
	return &voyageOutput{Body: dto.VoyageFromDB(voyage)}, nil
}
