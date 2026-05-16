package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type OrgTripHandler struct {
	q  sqlcdb.Querier
	db *sql.DB
}

func NewOrgTripHandler(q sqlcdb.Querier, db *sql.DB) *OrgTripHandler {
	return &OrgTripHandler{q: q, db: db}
}

type orgTripParam struct {
	Slug string `path:"slug" doc:"Organization slug"`
	ID   int64  `path:"tripID" doc:"Trip ID"`
}

type createOrgTripInput struct {
	Slug string `path:"slug" doc:"Organization slug"`
	Body dto.TripBody
}

type updateOrgTripInput struct {
	Slug string `path:"slug" doc:"Organization slug"`
	ID   int64  `path:"tripID" doc:"Trip ID"`
	Body dto.TripBody
}

type completeOrgTripInput struct {
	Slug string `path:"slug" doc:"Organization slug"`
	ID   int64  `path:"tripID" doc:"Trip ID"`
	Body dto.CompleteTripBody
}

// RegisterOrgTripRoutes wires the org-scoped trip operations onto the API.
func RegisterOrgTripRoutes(api huma.API, q sqlcdb.Querier, db *sql.DB) {
	h := NewOrgTripHandler(q, db)
	tag := []string{"Org trips"}

	huma.Register(api, huma.Operation{
		OperationID: "list-org-trips", Method: http.MethodGet, Path: "/orgs/{slug}/trips",
		Summary: "List org trips", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-org-trip", Method: http.MethodGet, Path: "/orgs/{slug}/trips/{tripID}",
		Summary: "Get an org trip", Tags: tag,
	}, h.get)
	huma.Register(api, huma.Operation{
		OperationID: "create-org-trip", Method: http.MethodPost, Path: "/orgs/{slug}/trips",
		Summary: "Create an org trip (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-org-trip", Method: http.MethodPut, Path: "/orgs/{slug}/trips/{tripID}",
		Summary: "Update an org trip (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-org-trip", Method: http.MethodDelete, Path: "/orgs/{slug}/trips/{tripID}",
		Summary: "Delete an org trip (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
	huma.Register(api, huma.Operation{
		OperationID: "cancel-org-trip", Method: http.MethodPost, Path: "/orgs/{slug}/trips/{tripID}/cancel",
		Summary: "Cancel an org trip (admin)", Tags: tag,
	}, h.cancel)
	huma.Register(api, huma.Operation{
		OperationID: "complete-org-trip", Method: http.MethodPost, Path: "/orgs/{slug}/trips/{tripID}/complete",
		Summary: "Complete an org trip into a voyage (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.complete)
}

func (h *OrgTripHandler) list(ctx context.Context, in *orgListParams) (*tripListOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	id := orgID(octx)
	trips, err := h.q.ListOrgTrips(ctx, sqlcdb.ListOrgTripsParams{
		OrgID:  id,
		Limit:  in.Limit,
		Offset: in.Offset,
	})
	if err != nil {
		slog.Error("list org trips", "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list trips")
	}
	total, err := h.q.CountOrgTrips(ctx, id)
	if err != nil {
		slog.Error("count org trips", "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list trips")
	}
	return &tripListOutput{Body: dto.NewPage(dto.TripsFromDB(trips), total, in.Limit, in.Offset)}, nil
}

func (h *OrgTripHandler) get(ctx context.Context, in *orgTripParam) (*tripOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	trip, err := h.q.GetOrgTrip(ctx, sqlcdb.GetOrgTripParams{ID: in.ID, OrgID: orgID(octx)})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found")
		}
		slog.Error("get org trip", "trip_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get trip")
	}
	return &tripOutput{Body: dto.TripFromDB(trip)}, nil
}

func (h *OrgTripHandler) create(ctx context.Context, in *createOrgTripInput) (*tripOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	user := middleware.GetUser(ctx)
	trip, err := h.q.CreateOrgTrip(ctx, sqlcdb.CreateOrgTripParams{
		OwnerID:       user.UserID,
		OrgID:         orgID(octx),
		CruiseID:      nullInt64(in.Body.CruiseID),
		Name:          in.Body.Name,
		EmbarkDate:    nullString(in.Body.EmbarkDate),
		DisembarkDate: nullString(in.Body.DisembarkDate),
		Countries:     nullString(in.Body.Countries),
		StartPort:     nullString(in.Body.StartPort),
		EndPort:       nullString(in.Body.EndPort),
		CaptainName:   nullString(in.Body.CaptainName),
		YachtID:       nullInt64(in.Body.YachtID),
		CostTotal:     nullFloat64(in.Body.CostTotal),
		CostPerPerson: nullFloat64(in.Body.CostPerPerson),
		MaxCrew:       nullInt64(in.Body.MaxCrew),
		ImageLogoUrl:  nullString(in.Body.ImageLogoUrl),
		ImagePhotoUrl: nullString(in.Body.ImagePhotoUrl),
		ImageRouteUrl: nullString(in.Body.ImageRouteUrl),
		Description:   nullString(in.Body.Description),
		Status:        sqlcdb.TripStatusPlanned,
	})
	if err != nil {
		slog.Error("create org trip", "org_id", octx.OrgID, "user_id", user.UserID, "name", in.Body.Name, "err", err)
		return nil, huma.Error500InternalServerError("failed to create trip")
	}
	return &tripOutput{Body: dto.TripFromDB(trip)}, nil
}

func (h *OrgTripHandler) update(ctx context.Context, in *updateOrgTripInput) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.UpdateOrgTrip(ctx, sqlcdb.UpdateOrgTripParams{
		Name:          in.Body.Name,
		EmbarkDate:    nullString(in.Body.EmbarkDate),
		DisembarkDate: nullString(in.Body.DisembarkDate),
		Countries:     nullString(in.Body.Countries),
		StartPort:     nullString(in.Body.StartPort),
		EndPort:       nullString(in.Body.EndPort),
		CaptainName:   nullString(in.Body.CaptainName),
		YachtID:       nullInt64(in.Body.YachtID),
		CostTotal:     nullFloat64(in.Body.CostTotal),
		CostPerPerson: nullFloat64(in.Body.CostPerPerson),
		MaxCrew:       nullInt64(in.Body.MaxCrew),
		ImageLogoUrl:  nullString(in.Body.ImageLogoUrl),
		ImagePhotoUrl: nullString(in.Body.ImagePhotoUrl),
		ImageRouteUrl: nullString(in.Body.ImageRouteUrl),
		Description:   nullString(in.Body.Description),
		CruiseID:      nullInt64(in.Body.CruiseID),
		ID:            in.ID,
		OrgID:         orgID(octx),
	}); err != nil {
		slog.Error("update org trip", "trip_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update trip")
	}
	return &noContentOutput{}, nil
}

func (h *OrgTripHandler) delete(ctx context.Context, in *orgTripParam) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.DeleteOrgTrip(ctx, sqlcdb.DeleteOrgTripParams{ID: in.ID, OrgID: orgID(octx)}); err != nil {
		slog.Error("delete org trip", "trip_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete trip")
	}
	return &noContentOutput{}, nil
}

func (h *OrgTripHandler) cancel(ctx context.Context, in *orgTripParam) (*tripOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	trip, err := h.q.CancelOrgTrip(ctx, sqlcdb.CancelOrgTripParams{ID: in.ID, OrgID: orgID(octx)})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found or invalid transition")
		}
		slog.Error("cancel org trip", "trip_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to cancel trip")
	}
	return &tripOutput{Body: dto.TripFromDB(trip)}, nil
}

func (h *OrgTripHandler) complete(ctx context.Context, in *completeOrgTripInput) (*voyageOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	user := middleware.GetUser(ctx)
	voyage, err := completeTripTx(ctx, h.db, orgID(octx), in.ID, user.UserID, in.Body)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found or not in planned state")
		}
		slog.Error("complete org trip", "trip_id", in.ID, "org_id", octx.OrgID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to complete trip")
	}
	return &voyageOutput{Body: dto.VoyageFromDB(voyage)}, nil
}
