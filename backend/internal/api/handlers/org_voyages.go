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

type OrgVoyageHandler struct {
	q sqlcdb.Querier
}

func NewOrgVoyageHandler(q sqlcdb.Querier) *OrgVoyageHandler {
	return &OrgVoyageHandler{q: q}
}

type orgVoyageParam struct {
	Slug string `path:"slug" doc:"Organization slug"`
	ID   int64  `path:"voyageID" doc:"Voyage ID"`
}

type createOrgVoyageInput struct {
	Slug string `path:"slug" doc:"Organization slug"`
	Body dto.VoyageBody
}

type updateOrgVoyageInput struct {
	Slug string `path:"slug" doc:"Organization slug"`
	ID   int64  `path:"voyageID" doc:"Voyage ID"`
	Body dto.VoyageBody
}

// RegisterOrgVoyageRoutes wires the org-scoped voyage operations onto the API.
func RegisterOrgVoyageRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewOrgVoyageHandler(q)
	tag := []string{"Org voyages"}

	huma.Register(api, huma.Operation{
		OperationID: "list-org-voyages", Method: http.MethodGet, Path: "/orgs/{slug}/voyages",
		Summary: "List org voyages", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-org-voyage", Method: http.MethodGet, Path: "/orgs/{slug}/voyages/{voyageID}",
		Summary: "Get an org voyage", Tags: tag,
	}, h.get)
	huma.Register(api, huma.Operation{
		OperationID: "create-org-voyage", Method: http.MethodPost, Path: "/orgs/{slug}/voyages",
		Summary: "Create an org voyage (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-org-voyage", Method: http.MethodPut, Path: "/orgs/{slug}/voyages/{voyageID}",
		Summary: "Update an org voyage (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-org-voyage", Method: http.MethodDelete, Path: "/orgs/{slug}/voyages/{voyageID}",
		Summary: "Delete an org voyage (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
}

func (h *OrgVoyageHandler) list(ctx context.Context, in *orgSlugParam) (*voyageListOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	voyages, err := h.q.ListOrgVoyages(ctx, orgID(octx))
	if err != nil {
		slog.Error("list org voyages", "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list voyages")
	}
	return &voyageListOutput{Body: dto.VoyagesFromDB(voyages)}, nil
}

func (h *OrgVoyageHandler) get(ctx context.Context, in *orgVoyageParam) (*voyageOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	voyage, err := h.q.GetOrgVoyage(ctx, sqlcdb.GetOrgVoyageParams{ID: in.ID, OrgID: orgID(octx)})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("voyage not found")
		}
		slog.Error("get org voyage", "voyage_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get voyage")
	}
	return &voyageOutput{Body: dto.VoyageFromDB(voyage)}, nil
}

func (h *OrgVoyageHandler) create(ctx context.Context, in *createOrgVoyageInput) (*voyageOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	user := middleware.GetUser(ctx)
	voyage, err := h.q.CreateOrgVoyage(ctx, sqlcdb.CreateOrgVoyageParams{
		OwnerID:       user.UserID,
		OrgID:         orgID(octx),
		CruiseID:      nullInt64(in.Body.CruiseID),
		Name:          in.Body.Name,
		Year:          nullInt64(in.Body.Year),
		EmbarkDate:    nullString(in.Body.EmbarkDate),
		DisembarkDate: nullString(in.Body.DisembarkDate),
		Countries:     nullString(in.Body.Countries),
		StartPort:     nullString(in.Body.StartPort),
		EndPort:       nullString(in.Body.EndPort),
		CaptainName:   nullString(in.Body.CaptainName),
		YachtID:       nullInt64(in.Body.YachtID),
		HoursTotal:    valOrZeroFloat(in.Body.HoursTotal),
		HoursSail:     valOrZeroFloat(in.Body.HoursSail),
		HoursEngine:   valOrZeroFloat(in.Body.HoursEngine),
		HoursOver6bf:  valOrZeroFloat(in.Body.HoursOver6bf),
		Miles:         valOrZeroFloat(in.Body.Miles),
		Days:          valOrZeroInt(in.Body.Days),
		TidalWaters:   valOrZeroInt(in.Body.TidalWaters),
		CostTotal:     nullFloat64(in.Body.CostTotal),
		CostPerPerson: nullFloat64(in.Body.CostPerPerson),
		ImageLogoUrl:  nullString(in.Body.ImageLogoUrl),
		ImagePhotoUrl: nullString(in.Body.ImagePhotoUrl),
		ImageRouteUrl: nullString(in.Body.ImageRouteUrl),
		Description:   nullString(in.Body.Description),
	})
	if err != nil {
		slog.Error("create org voyage", "org_id", octx.OrgID, "user_id", user.UserID, "name", in.Body.Name, "err", err)
		return nil, huma.Error500InternalServerError("failed to create voyage")
	}
	return &voyageOutput{Body: dto.VoyageFromDB(voyage)}, nil
}

func (h *OrgVoyageHandler) update(ctx context.Context, in *updateOrgVoyageInput) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.UpdateOrgVoyage(ctx, sqlcdb.UpdateOrgVoyageParams{
		Name:          in.Body.Name,
		Year:          nullInt64(in.Body.Year),
		EmbarkDate:    nullString(in.Body.EmbarkDate),
		DisembarkDate: nullString(in.Body.DisembarkDate),
		Countries:     nullString(in.Body.Countries),
		StartPort:     nullString(in.Body.StartPort),
		EndPort:       nullString(in.Body.EndPort),
		CaptainName:   nullString(in.Body.CaptainName),
		YachtID:       nullInt64(in.Body.YachtID),
		HoursTotal:    valOrZeroFloat(in.Body.HoursTotal),
		HoursSail:     valOrZeroFloat(in.Body.HoursSail),
		HoursEngine:   valOrZeroFloat(in.Body.HoursEngine),
		HoursOver6bf:  valOrZeroFloat(in.Body.HoursOver6bf),
		Miles:         valOrZeroFloat(in.Body.Miles),
		Days:          valOrZeroInt(in.Body.Days),
		TidalWaters:   valOrZeroInt(in.Body.TidalWaters),
		CostTotal:     nullFloat64(in.Body.CostTotal),
		CostPerPerson: nullFloat64(in.Body.CostPerPerson),
		ImageLogoUrl:  nullString(in.Body.ImageLogoUrl),
		ImagePhotoUrl: nullString(in.Body.ImagePhotoUrl),
		ImageRouteUrl: nullString(in.Body.ImageRouteUrl),
		Description:   nullString(in.Body.Description),
		CruiseID:      nullInt64(in.Body.CruiseID),
		ID:            in.ID,
		OrgID:         orgID(octx),
	}); err != nil {
		slog.Error("update org voyage", "voyage_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update voyage")
	}
	return &noContentOutput{}, nil
}

func (h *OrgVoyageHandler) delete(ctx context.Context, in *orgVoyageParam) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.DeleteOrgVoyage(ctx, sqlcdb.DeleteOrgVoyageParams{ID: in.ID, OrgID: orgID(octx)}); err != nil {
		slog.Error("delete org voyage", "voyage_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete voyage")
	}
	return &noContentOutput{}, nil
}
