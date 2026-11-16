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
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type VoyageHandler struct {
	q sqlcdb.Querier
}

func NewVoyageHandler(q sqlcdb.Querier) *VoyageHandler {
	return &VoyageHandler{q: q}
}

func (h *VoyageHandler) updateLog(ctx context.Context, in *updateVoyageLogInput) (*noContentOutput, error) {
	if _, err := getVoyageForCaptain(ctx, h.q, in.ID); err != nil {
		return nil, err
	}
	callerID, admin := captainCaller(ctx)
	n, err := h.q.UpdateVoyageLog(ctx, sqlcdb.UpdateVoyageLogParams{
		HoursTotal:    valOrZeroFloat(in.Body.HoursTotal),
		HoursSail:     valOrZeroFloat(in.Body.HoursSail),
		HoursEngine:   valOrZeroFloat(in.Body.HoursEngine),
		HoursOver6bf:  valOrZeroFloat(in.Body.HoursOver6bf),
		Miles:         valOrZeroFloat(in.Body.Miles),
		Days:          valOrZeroInt(in.Body.Days),
		TidalWaters:   valOrZeroInt(in.Body.TidalWaters),
		CaptainNotes:  nullString(in.Body.CaptainNotes),
		YachtID:       nullInt64(in.Body.YachtID),
		TrackUrl:      nullString(trimmedPtr(in.Body.TrackUrl)),
		VoyageCard:    nullString(trimmedPtr(in.Body.VoyageCard)),
		ID:            in.ID,
		CallerIsAdmin: admin,
		CallerID:      callerID,
	})
	if err != nil {
		if isForeignKeyViolation(err, "voyages_yacht_id_fkey") {
			return nil, yachtNotFound()
		}
		slog.Error("update voyage log", "voyage_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update voyage log")
	}
	if n == 0 {
		return nil, refusedCaptainWrite(ctx, h.q, in.ID, errAdminOrCaptainOnly())
	}
	return &noContentOutput{}, nil
}

// addYacht creates a yacht from a voyage form so a charter yacht used once
// can be entered without leaving the form. The voyage's captain may add one
// here even though the club yacht list is otherwise admin-managed; picking it
// for the voyage is a separate log or voyage update.
func (h *VoyageHandler) addYacht(ctx context.Context, in *addVoyageYachtInput) (*yachtOutput, error) {
	if _, err := getVoyageForCaptain(ctx, h.q, in.ID); err != nil {
		return nil, err
	}
	callerID, admin := captainCaller(ctx)
	yacht, err := h.q.AddVoyageYacht(ctx, sqlcdb.AddVoyageYachtParams{
		CallerID:       callerID,
		CallerIsAdmin:  admin,
		VoyageID:       in.ID,
		Name:           in.Body.Name,
		RegistrationNo: nullString(in.Body.RegistrationNo),
		YachtType:      nullString(in.Body.YachtType),
		Berths:         nullInt64(in.Body.Berths),
		LengthM:        nullFloat64(in.Body.LengthM),
		BuildYear:      nullInt64(in.Body.BuildYear),
		Engine:         nullString(in.Body.Engine),
		OwnerName:      nullString(in.Body.OwnerName),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, refusedCaptainWrite(ctx, h.q, in.ID, errAdminOrCaptainOnly())
	}
	if err != nil {
		slog.Error("add voyage yacht", "voyage_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to add yacht")
	}
	return &yachtOutput{Body: dto.YachtFromDB(yacht)}, nil
}

// getVoyageForCaptain loads a voyage and authorizes the caller as an admin or
// its assigned captain. A hidden voyage stays writable by its assigned
// captain, as publication does not change who may edit a record; any other
// caller who cannot see it gets 404, not 403.
func getVoyageForCaptain(ctx context.Context, q sqlcdb.Querier, id int64) (sqlcdb.Voyage, error) {
	voyage, err := q.GetVoyage(ctx, sqlcdb.GetVoyageParams{ID: id, IncludeHidden: true})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlcdb.Voyage{}, huma.Error404NotFound("voyage not found")
		}
		slog.Error("get voyage", "voyage_id", id, "err", err)
		return sqlcdb.Voyage{}, huma.Error500InternalServerError("failed to get voyage")
	}
	if authErr := requireAdminOrCaptain(ctx, q, voyage.CaptainPersonID); authErr != nil {
		return sqlcdb.Voyage{}, hideForbidden(ctx, authErr, func() error {
			return requireVisibleVoyage(ctx, q, id)
		})
	}
	return voyage, nil
}

// captainCaller returns what a captain write statement checks the caller
// by: the account ID and whether it is an admin. Those statements write
// only for an admin or the account linked to the voyage's captain person,
// so a captain change committed after getVoyageForCaptain still refuses
// the write without locking the voyage.
func captainCaller(ctx context.Context) (userID int64, admin bool) {
	u := middleware.GetUser(ctx)
	if u == nil {
		return 0, false
	}
	return u.UserID, u.Role == "admin"
}

// refusedCaptainWrite answers a captain write whose statement changed
// nothing because the caller no longer passed its captain check. The
// voyage is authorized again to pick 403 or, for a hidden voyage the
// caller cannot see, 404; a caller who passes again by then gets fallback.
func refusedCaptainWrite(ctx context.Context, q sqlcdb.Querier, id int64, fallback error) error {
	if _, err := getVoyageForCaptain(ctx, q, id); err != nil {
		return err
	}
	return fallback
}

// errAdminOrCaptainOnly is requireAdminOrCaptain's refusal.
func errAdminOrCaptainOnly() error {
	return huma.Error403Forbidden("admin or captain only")
}

type voyageIDParam struct {
	ID int64 `path:"voyageID" doc:"Voyage ID"`
}

type createVoyageInput struct {
	Body dto.VoyageBody
}

type updateVoyageInput struct {
	ID   int64 `path:"voyageID" doc:"Voyage ID"`
	Body dto.VoyageBody
}

type voyageListOutput struct {
	Body dto.Page[dto.Voyage]
}

// cruiseVoyagesOutput is the unpaginated array body for a cruise's child voyages.
type cruiseVoyagesOutput struct {
	Body []dto.Voyage
}

// voyageDetailOutput is a single voyage with its yacht's summary.
type voyageDetailOutput struct {
	Body dto.VoyageDetail
}

// listVoyageYachtsInput repeats the pageParams fields: huma does not apply
// the defaults of an embedded struct.
type listVoyageYachtsInput struct {
	ID     int64  `path:"voyageID" doc:"Voyage ID"`
	Limit  int32  `query:"limit" minimum:"1" maximum:"100" default:"50" doc:"Maximum number of items to return"`
	Offset int32  `query:"offset" minimum:"0" default:"0" doc:"Number of items to skip"`
	After  string `query:"after" doc:"A previous page's next_cursor: the window starts after that page's last item, so rows deleted meanwhile shift nothing"`
}

type addVoyageYachtInput struct {
	ID   int64 `path:"voyageID" doc:"Voyage ID"`
	Body dto.YachtBody
}

type updateVoyageLogInput struct {
	ID   int64 `path:"voyageID" doc:"Voyage ID"`
	Body dto.VoyageLogBody
}

// RegisterVoyageRoutes wires the club voyage operations onto the API. Reads are
// open to any member; mutations require an admin, except the post-cruise log
// and its yacht picker, which the voyage's captain may also use.
func RegisterVoyageRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewVoyageHandler(q)
	tag := []string{"Voyages"}

	huma.Register(api, huma.Operation{
		OperationID: "list-voyages", Method: http.MethodGet, Path: "/voyages",
		Summary: "List voyages", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-voyage", Method: http.MethodGet, Path: "/voyages/{voyageID}",
		Summary: "Get a voyage", Tags: tag,
	}, h.get)
	huma.Register(api, huma.Operation{
		OperationID: "create-voyage", Method: http.MethodPost, Path: "/voyages",
		Summary: "Create a voyage (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-voyage", Method: http.MethodPut, Path: "/voyages/{voyageID}",
		Summary: "Update a voyage (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "update-voyage-log", Method: http.MethodPut, Path: "/voyages/{voyageID}/log",
		Summary: "Log post-cruise statistics and notes (admin or voyage captain)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.updateLog)
	huma.Register(api, huma.Operation{
		OperationID: "list-voyage-yachts", Method: http.MethodGet, Path: "/voyages/{voyageID}/yachts",
		Summary: "List the club yachts for the voyage log (admin or voyage captain)", Tags: tag,
	}, h.listYachts)
	huma.Register(api, huma.Operation{
		OperationID: "add-voyage-yacht", Method: http.MethodPost, Path: "/voyages/{voyageID}/yachts",
		Summary: "Add a yacht to the club list from a voyage (admin or voyage captain)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.addYacht)
	huma.Register(api, huma.Operation{
		OperationID: "delete-voyage", Method: http.MethodDelete, Path: "/voyages/{voyageID}",
		Summary: "Delete a voyage (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
}

// voyageWriteErr maps a voyage insert or update that references a captain
// person or yacht deleted underneath it to a client error.
func voyageWriteErr(err error) error {
	switch {
	case isForeignKeyViolation(err, "voyages_captain_person_id_fkey"):
		return captainPersonNotFound()
	case isForeignKeyViolation(err, "voyages_yacht_id_fkey"):
		return yachtNotFound()
	}
	return err
}

// listVoyagePage lists one window of voyages, after the in.After cursor
// when one is given.
func listVoyagePage(ctx context.Context, q sqlcdb.Querier, in *pageParams) ([]sqlcdb.Voyage, error) {
	var key voyageKey
	ok, err := decodeCursor(in.After, &key)
	if err != nil {
		return nil, err
	}
	year := types.NullInt64{}
	if key.Year != nil {
		year = types.NullInt64{Int64: *key.Year, Valid: true}
	}
	return q.ListVoyages(ctx, sqlcdb.ListVoyagesParams{
		IncludeHidden: canSeeHidden(ctx),
		AfterID:       afterID(ok, key.ID), AfterYear: year, AfterEmbarkDate: afterNullText(key.Date),
		RowLimit: fetchLimit(in.Limit), RowOffset: in.Offset,
	})
}

// voyageCursor is a voyage's keyset sort key: (year, embark_date, id).
func voyageCursor(r sqlcdb.Voyage) any {
	k := voyageKey{Date: nullTextKey(r.EmbarkDate), ID: r.ID}
	if r.Year.Valid {
		k.Year = &r.Year.Int64
	}
	return k
}

func (h *VoyageHandler) list(ctx context.Context, in *pageParams) (*voyageListOutput, error) {
	scope, _ := memberScope(ctx)
	rows, err := listVoyagePage(ctx, h.q, in)
	if se, ok := errors.AsType[huma.StatusError](err); ok {
		return nil, se
	}
	if err != nil {
		logCRUD("list voyages", scopeAttrs(scope), err)
		return nil, huma.Error500InternalServerError("failed to list voyages")
	}
	total, err := h.q.CountVoyages(ctx, canSeeHidden(ctx))
	if err != nil {
		logCRUD("list voyages", scopeAttrs(scope), err)
		return nil, huma.Error500InternalServerError("failed to list voyages")
	}
	return &voyageListOutput{Body: buildPage(rows, total, in.Limit, in.Offset, dto.VoyagesFromDB, voyageCursor)}, nil
}

func (h *VoyageHandler) get(ctx context.Context, in *voyageIDParam) (*voyageDetailOutput, error) {
	scope, _ := memberScope(ctx)
	row, err := h.q.GetVoyage(ctx, sqlcdb.GetVoyageParams{ID: in.ID, IncludeHidden: canSeeHidden(ctx)})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("voyage not found")
		}
		logCRUD("get voyage", scopeAttrs(scope, "voyage_id", in.ID), err)
		return nil, huma.Error500InternalServerError("failed to get voyage")
	}
	out := &voyageDetailOutput{Body: dto.VoyageDetail{Voyage: dto.VoyageFromDB(row)}}
	if !row.YachtID.Valid {
		return out, nil
	}
	yacht, err := h.q.GetYacht(ctx, row.YachtID.Int64)
	switch {
	case err == nil:
		summary := dto.YachtSummaryFromDB(yacht)
		out.Body.Yacht = &summary
	case errors.Is(err, sql.ErrNoRows):
		// The voyage changed yacht and the old one was deleted after the read.
	default:
		logCRUD("get voyage yacht", scopeAttrs(scope, "voyage_id", in.ID, "yacht_id", row.YachtID.Int64), err)
		return nil, huma.Error500InternalServerError("failed to get voyage")
	}
	return out, nil
}

// listYachts lists the club yachts for the voyage log's yacht picker. The
// voyage's captain may pick from the list here even outside the captain
// group, which alone reads GET /yachts. It is a read nested under the
// voyage, so a hidden voyage answers 404 to a caller who cannot see it.
func (h *VoyageHandler) listYachts(ctx context.Context, in *listVoyageYachtsInput) (*yachtListOutput, error) {
	if err := requireVisibleVoyage(ctx, h.q, in.ID); err != nil {
		return nil, err
	}
	if _, err := getVoyageForCaptain(ctx, h.q, in.ID); err != nil {
		return nil, err
	}
	page := &pageParams{Limit: in.Limit, Offset: in.Offset, After: in.After}
	rows, err := listYachtPage(ctx, h.q, page)
	if se, ok := errors.AsType[huma.StatusError](err); ok {
		return nil, se
	}
	if err != nil {
		slog.Error("list voyage yachts", "voyage_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list yachts")
	}
	total, err := h.q.CountYachts(ctx)
	if err != nil {
		slog.Error("list voyage yachts", "voyage_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list yachts")
	}
	return yachtListPage(page, rows, total), nil
}

func (h *VoyageHandler) create(ctx context.Context, in *createVoyageInput) (*voyageOutput, error) {
	scope, err := adminScope(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireParentCruise(ctx, h.q, in.Body.CruiseID); err != nil {
		return nil, voyageMutationError(err, "create voyage", "failed to create voyage", scopeAttrs(scope, "name", in.Body.Name))
	}
	params := createVoyageParams(scope, in.Body)
	if err := retainUploadImages(ctx, h.q, in.Body.ImageLogoUrl, in.Body.ImagePhotoUrl, in.Body.ImageRouteUrl); err != nil {
		return nil, voyageMutationError(err, "create voyage", "failed to create voyage", scopeAttrs(scope, "name", in.Body.Name))
	}
	params.CaptainPersonID, params.CaptainName, err = captainAssignment(ctx, h.q, in.Body.CaptainPersonID, in.Body.CaptainName)
	if err != nil {
		return nil, voyageMutationError(err, "create voyage", "failed to create voyage", scopeAttrs(scope, "name", in.Body.Name))
	}
	voyage, err := h.q.CreateVoyage(ctx, params)
	if err := voyageWriteErr(err); err != nil {
		return nil, voyageMutationError(err, "create voyage", "failed to create voyage", scopeAttrs(scope, "name", in.Body.Name))
	}
	return &voyageOutput{Body: dto.VoyageFromDB(voyage)}, nil
}

func (h *VoyageHandler) update(ctx context.Context, in *updateVoyageInput) (*noContentOutput, error) {
	scope, err := adminScope(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireParentCruise(ctx, h.q, in.Body.CruiseID); err != nil {
		return nil, voyageMutationError(err, "update voyage", "failed to update voyage", scopeAttrs(scope, "voyage_id", in.ID))
	}
	params := updateVoyageParams(in.Body)
	params.ID = in.ID
	if _, err := h.q.GetVoyage(ctx, sqlcdb.GetVoyageParams{ID: in.ID, IncludeHidden: true}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("voyage not found")
		}
		return nil, voyageMutationError(err, "update voyage", "failed to update voyage", scopeAttrs(scope, "voyage_id", in.ID))
	}
	params.CaptainPersonID, params.CaptainName, err = captainAssignment(ctx, h.q, in.Body.CaptainPersonID, in.Body.CaptainName)
	if err != nil {
		return nil, voyageMutationError(err, "update voyage", "failed to update voyage", scopeAttrs(scope, "voyage_id", in.ID))
	}
	if err := retainUploadImages(ctx, h.q, in.Body.ImageLogoUrl, in.Body.ImagePhotoUrl, in.Body.ImageRouteUrl); err != nil {
		return nil, voyageMutationError(err, "update voyage", "failed to update voyage", scopeAttrs(scope, "voyage_id", in.ID))
	}
	if err := voyageWriteErr(h.q.UpdateVoyage(ctx, params)); err != nil {
		return nil, voyageMutationError(err, "update voyage", "failed to update voyage", scopeAttrs(scope, "voyage_id", in.ID))
	}
	return &noContentOutput{}, nil
}

func (h *VoyageHandler) delete(ctx context.Context, in *voyageIDParam) (*noContentOutput, error) {
	scope, err := adminScope(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.q.DeleteVoyage(ctx, in.ID); err != nil {
		return nil, voyageMutationError(err, "delete voyage", "failed to delete voyage", scopeAttrs(scope, "voyage_id", in.ID))
	}
	return &noContentOutput{}, nil
}

func voyageMutationError(err error, logMsg, clientMsg string, attrs []any) error {
	if se, ok := errors.AsType[huma.StatusError](err); ok {
		return se
	}
	logCRUD(logMsg, attrs, err)
	return huma.Error500InternalServerError(clientMsg)
}

func createVoyageParams(scope crudScope, body dto.VoyageBody) sqlcdb.CreateVoyageParams {
	start, end := legPort(body.StartPort), legPort(body.EndPort)
	return sqlcdb.CreateVoyageParams{
		CreatedBy:          types.NullInt64{Int64: scope.userID, Valid: true},
		CruiseID:           body.CruiseID,
		Name:               body.Name,
		Year:               nullInt64(body.Year),
		EmbarkDate:         nullString(body.EmbarkDate),
		DisembarkDate:      nullString(body.DisembarkDate),
		Countries:          body.Countries.Stored(),
		StartPort:          start.name,
		StartPortLatitude:  start.lat,
		StartPortLongitude: start.lon,
		EndPort:            end.name,
		EndPortLatitude:    end.lat,
		EndPortLongitude:   end.lon,
		CaptainName:        nullString(body.CaptainName),
		YachtID:            nullInt64(body.YachtID),
		HoursTotal:         valOrZeroFloat(body.HoursTotal),
		HoursSail:          valOrZeroFloat(body.HoursSail),
		HoursEngine:        valOrZeroFloat(body.HoursEngine),
		HoursOver6bf:       valOrZeroFloat(body.HoursOver6bf),
		Miles:              valOrZeroFloat(body.Miles),
		Days:               valOrZeroInt(body.Days),
		TidalWaters:        valOrZeroInt(body.TidalWaters),
		CostTotal:          nullFloat64(body.CostTotal),
		CostPerPerson:      nullFloat64(body.CostPerPerson),
		ImageLogoUrl:       nullString(body.ImageLogoUrl),
		ImagePhotoUrl:      nullString(body.ImagePhotoUrl),
		ImageRouteUrl:      nullString(body.ImageRouteUrl),
		Description:        nullString(body.Description),
		TrackUrl:           nullString(trimmedPtr(body.TrackUrl)),
		VoyageCard:         nullString(trimmedPtr(body.VoyageCard)),
	}
}

func updateVoyageParams(body dto.VoyageBody) sqlcdb.UpdateVoyageParams {
	start, end := legPort(body.StartPort), legPort(body.EndPort)
	return sqlcdb.UpdateVoyageParams{
		Name:               body.Name,
		Year:               nullInt64(body.Year),
		EmbarkDate:         nullString(body.EmbarkDate),
		DisembarkDate:      nullString(body.DisembarkDate),
		Countries:          body.Countries.Stored(),
		StartPort:          start.name,
		StartPortLatitude:  start.lat,
		StartPortLongitude: start.lon,
		EndPort:            end.name,
		EndPortLatitude:    end.lat,
		EndPortLongitude:   end.lon,
		CaptainName:        nullString(body.CaptainName),
		YachtID:            nullInt64(body.YachtID),
		HoursTotal:         valOrZeroFloat(body.HoursTotal),
		HoursSail:          valOrZeroFloat(body.HoursSail),
		HoursEngine:        valOrZeroFloat(body.HoursEngine),
		HoursOver6bf:       valOrZeroFloat(body.HoursOver6bf),
		Miles:              valOrZeroFloat(body.Miles),
		Days:               valOrZeroInt(body.Days),
		TidalWaters:        valOrZeroInt(body.TidalWaters),
		CostTotal:          nullFloat64(body.CostTotal),
		CostPerPerson:      nullFloat64(body.CostPerPerson),
		ImageLogoUrl:       nullString(body.ImageLogoUrl),
		ImagePhotoUrl:      nullString(body.ImagePhotoUrl),
		ImageRouteUrl:      nullString(body.ImageRouteUrl),
		Description:        nullString(body.Description),
		TrackUrl:           nullString(trimmedPtr(body.TrackUrl)),
		VoyageCard:         nullString(trimmedPtr(body.VoyageCard)),
		CruiseID:           body.CruiseID,
	}
}
