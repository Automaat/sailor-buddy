package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type OrgCruiseHandler struct {
	q sqlcdb.Querier
}

func NewOrgCruiseHandler(q sqlcdb.Querier) *OrgCruiseHandler {
	return &OrgCruiseHandler{q: q}
}

type orgCruiseParam struct {
	Slug string `path:"slug" doc:"Organization slug"`
	ID   int64  `path:"cruiseID" doc:"Cruise ID"`
}

type createOrgCruiseInput struct {
	Slug string `path:"slug" doc:"Organization slug"`
	Body dto.CruiseBody
}

type updateOrgCruiseInput struct {
	Slug string `path:"slug" doc:"Organization slug"`
	ID   int64  `path:"cruiseID" doc:"Cruise ID"`
	Body dto.CruiseBody
}

type cruiseOutput struct {
	Body dto.Cruise
}

type cruiseListOutput struct {
	Body dto.Page[dto.Cruise]
}

// RegisterOrgCruiseRoutes wires the org-scoped cruise operations onto the API.
func RegisterOrgCruiseRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewOrgCruiseHandler(q)
	tag := []string{"Org cruises"}

	huma.Register(api, huma.Operation{
		OperationID: "list-org-cruises", Method: http.MethodGet, Path: "/orgs/{slug}/cruises",
		Summary: "List org cruises", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-org-cruise", Method: http.MethodGet, Path: "/orgs/{slug}/cruises/{cruiseID}",
		Summary: "Get an org cruise", Tags: tag,
	}, h.get)
	huma.Register(api, huma.Operation{
		OperationID: "create-org-cruise", Method: http.MethodPost, Path: "/orgs/{slug}/cruises",
		Summary: "Create an org cruise (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-org-cruise", Method: http.MethodPut, Path: "/orgs/{slug}/cruises/{cruiseID}",
		Summary: "Update an org cruise (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-org-cruise", Method: http.MethodDelete, Path: "/orgs/{slug}/cruises/{cruiseID}",
		Summary: "Delete an org cruise (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
	huma.Register(api, huma.Operation{
		OperationID: "generate-cruise-enroll-token", Method: http.MethodPost, Path: "/orgs/{slug}/cruises/{cruiseID}/enroll-token",
		Summary: "Generate a cruise enrollment token (admin)", Tags: tag,
	}, h.generateEnrollToken)
	huma.Register(api, huma.Operation{
		OperationID: "clear-cruise-enroll-token", Method: http.MethodDelete, Path: "/orgs/{slug}/cruises/{cruiseID}/enroll-token",
		Summary: "Clear a cruise enrollment token (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.clearEnrollToken)
	huma.Register(api, huma.Operation{
		OperationID: "list-cruise-trips", Method: http.MethodGet, Path: "/orgs/{slug}/cruises/{cruiseID}/trips",
		Summary: "List a cruise's child trips", Tags: tag,
	}, h.listChildTrips)
	huma.Register(api, huma.Operation{
		OperationID: "list-cruise-voyages", Method: http.MethodGet, Path: "/orgs/{slug}/cruises/{cruiseID}/voyages",
		Summary: "List a cruise's child voyages", Tags: tag,
	}, h.listChildVoyages)
}

func (h *OrgCruiseHandler) list(ctx context.Context, in *orgListParams) (*cruiseListOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	cruises, err := h.q.ListCruises(ctx, sqlcdb.ListCruisesParams{
		OrgID:  octx.OrgID,
		Limit:  in.Limit,
		Offset: in.Offset,
	})
	if err != nil {
		slog.Error("list org cruises", "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list cruises")
	}
	total, err := h.q.CountCruises(ctx, octx.OrgID)
	if err != nil {
		slog.Error("count org cruises", "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list cruises")
	}
	return &cruiseListOutput{Body: dto.NewPage(dto.CruisesFromDB(cruises), total, in.Limit, in.Offset)}, nil
}

func (h *OrgCruiseHandler) get(ctx context.Context, in *orgCruiseParam) (*cruiseOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	cruise, err := h.q.GetCruise(ctx, sqlcdb.GetCruiseParams{ID: in.ID, OrgID: octx.OrgID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("cruise not found")
		}
		slog.Error("get org cruise", "cruise_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get cruise")
	}
	return &cruiseOutput{Body: dto.CruiseFromDB(cruise)}, nil
}

func (h *OrgCruiseHandler) create(ctx context.Context, in *createOrgCruiseInput) (*cruiseOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	cruise, err := h.q.CreateCruise(ctx, sqlcdb.CreateCruiseParams{
		OrgID:         octx.OrgID,
		Name:          in.Body.Name,
		EmbarkDate:    nullString(in.Body.EmbarkDate),
		DisembarkDate: nullString(in.Body.DisembarkDate),
		Countries:     nullString(in.Body.Countries),
		StartPort:     nullString(in.Body.StartPort),
		EndPort:       nullString(in.Body.EndPort),
		Description:   nullString(in.Body.Description),
		ImageLogoUrl:  nullString(in.Body.ImageLogoUrl),
		ImagePhotoUrl: nullString(in.Body.ImagePhotoUrl),
		ImageRouteUrl: nullString(in.Body.ImageRouteUrl),
		MaxCrew:       nullInt64(in.Body.MaxCrew),
		CostPerPerson: nullFloat64(in.Body.CostPerPerson),
	})
	if err != nil {
		slog.Error("create org cruise", "org_id", octx.OrgID, "name", in.Body.Name, "err", err)
		return nil, huma.Error500InternalServerError("failed to create cruise")
	}
	return &cruiseOutput{Body: dto.CruiseFromDB(cruise)}, nil
}

func (h *OrgCruiseHandler) update(ctx context.Context, in *updateOrgCruiseInput) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.UpdateCruise(ctx, sqlcdb.UpdateCruiseParams{
		Name:          in.Body.Name,
		EmbarkDate:    nullString(in.Body.EmbarkDate),
		DisembarkDate: nullString(in.Body.DisembarkDate),
		Countries:     nullString(in.Body.Countries),
		StartPort:     nullString(in.Body.StartPort),
		EndPort:       nullString(in.Body.EndPort),
		Description:   nullString(in.Body.Description),
		ImageLogoUrl:  nullString(in.Body.ImageLogoUrl),
		ImagePhotoUrl: nullString(in.Body.ImagePhotoUrl),
		ImageRouteUrl: nullString(in.Body.ImageRouteUrl),
		MaxCrew:       nullInt64(in.Body.MaxCrew),
		CostPerPerson: nullFloat64(in.Body.CostPerPerson),
		ID:            in.ID,
		OrgID:         octx.OrgID,
	}); err != nil {
		slog.Error("update org cruise", "cruise_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update cruise")
	}
	return &noContentOutput{}, nil
}

func (h *OrgCruiseHandler) delete(ctx context.Context, in *orgCruiseParam) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.DeleteCruise(ctx, sqlcdb.DeleteCruiseParams{ID: in.ID, OrgID: octx.OrgID}); err != nil {
		slog.Error("delete org cruise", "cruise_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete cruise")
	}
	return &noContentOutput{}, nil
}

func (h *OrgCruiseHandler) generateEnrollToken(ctx context.Context, in *orgCruiseParam) (*tokenOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if _, err := h.q.GetCruise(ctx, sqlcdb.GetCruiseParams{ID: in.ID, OrgID: octx.OrgID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("cruise not found")
		}
		slog.Error("verify cruise for token generation", "cruise_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to verify cruise")
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, huma.Error500InternalServerError("failed to generate token")
	}
	token := hex.EncodeToString(b)
	if err := h.q.SetCruiseEnrollToken(ctx, sqlcdb.SetCruiseEnrollTokenParams{
		EnrollToken: types.NullString{String: token, Valid: true},
		ID:          in.ID,
		OrgID:       octx.OrgID,
	}); err != nil {
		slog.Error("set cruise enroll token", "cruise_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to set token")
	}
	out := &tokenOutput{}
	out.Body.Token = token
	return out, nil
}

func (h *OrgCruiseHandler) clearEnrollToken(ctx context.Context, in *orgCruiseParam) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.ClearCruiseEnrollToken(ctx, sqlcdb.ClearCruiseEnrollTokenParams{ID: in.ID, OrgID: octx.OrgID}); err != nil {
		slog.Error("clear cruise enroll token", "cruise_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to clear token")
	}
	return &noContentOutput{}, nil
}

func (h *OrgCruiseHandler) listChildTrips(ctx context.Context, in *orgCruiseParam) (*cruiseTripsOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	if err := h.verifyCruise(ctx, in.ID, octx.OrgID); err != nil {
		return nil, err
	}
	trips, err := h.q.ListCruiseTrips(ctx, types.NullInt64{Int64: in.ID, Valid: true})
	if err != nil {
		slog.Error("list cruise child trips", "cruise_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list trips")
	}
	return &cruiseTripsOutput{Body: dto.TripsFromDB(trips)}, nil
}

func (h *OrgCruiseHandler) listChildVoyages(ctx context.Context, in *orgCruiseParam) (*cruiseVoyagesOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	if err := h.verifyCruise(ctx, in.ID, octx.OrgID); err != nil {
		return nil, err
	}
	voyages, err := h.q.ListCruiseVoyages(ctx, types.NullInt64{Int64: in.ID, Valid: true})
	if err != nil {
		slog.Error("list cruise child voyages", "cruise_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list voyages")
	}
	return &cruiseVoyagesOutput{Body: dto.VoyagesFromDB(voyages)}, nil
}

// verifyCruise confirms the cruise exists within the org.
func (h *OrgCruiseHandler) verifyCruise(ctx context.Context, cruiseID, orgIDVal int64) error {
	if _, err := h.q.GetCruise(ctx, sqlcdb.GetCruiseParams{ID: cruiseID, OrgID: orgIDVal}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return huma.Error404NotFound("cruise not found")
		}
		slog.Error("verify cruise", "cruise_id", cruiseID, "org_id", orgIDVal, "err", err)
		return huma.Error500InternalServerError("failed to verify cruise")
	}
	return nil
}
