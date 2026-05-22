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
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type CruiseHandler struct {
	q sqlcdb.Querier
}

func NewCruiseHandler(q sqlcdb.Querier) *CruiseHandler {
	return &CruiseHandler{q: q}
}

type cruiseIDParam struct {
	ID int64 `path:"cruiseID" doc:"Cruise ID"`
}

type createCruiseInput struct {
	Body dto.CruiseBody
}

type updateCruiseInput struct {
	ID   int64 `path:"cruiseID" doc:"Cruise ID"`
	Body dto.CruiseBody
}

type cruiseOutput struct {
	Body dto.Cruise
}

type cruiseListOutput struct {
	Body dto.Page[dto.Cruise]
}

type cruiseEnrollmentsParam struct {
	CruiseID int64 `path:"cruiseID" doc:"Cruise ID"`
}

type cruiseEnrollmentParam struct {
	CruiseID     int64 `path:"cruiseID" doc:"Cruise ID"`
	EnrollmentID int64 `path:"enrollmentID" doc:"Enrollment ID"`
}

type cruiseEnrollmentStatusInput struct {
	CruiseID     int64 `path:"cruiseID" doc:"Cruise ID"`
	EnrollmentID int64 `path:"enrollmentID" doc:"Enrollment ID"`
	Body         dto.EnrollmentStatusBody
}

type cruiseEnrollmentTripInput struct {
	CruiseID     int64 `path:"cruiseID" doc:"Cruise ID"`
	EnrollmentID int64 `path:"enrollmentID" doc:"Enrollment ID"`
	Body         dto.AssignTripBody
}

type cruiseEnrollmentsOutput struct {
	Body []dto.CruiseEnrollmentDetail
}

// RegisterCruiseRoutes wires the club cruise operations onto the API. Reads are
// open to any member; mutations require an admin.
func RegisterCruiseRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewCruiseHandler(q)
	tag := []string{"Cruises"}

	huma.Register(api, huma.Operation{
		OperationID: "list-cruises", Method: http.MethodGet, Path: "/cruises",
		Summary: "List cruises", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-cruise", Method: http.MethodGet, Path: "/cruises/{cruiseID}",
		Summary: "Get a cruise", Tags: tag,
	}, h.get)
	huma.Register(api, huma.Operation{
		OperationID: "create-cruise", Method: http.MethodPost, Path: "/cruises",
		Summary: "Create a cruise (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-cruise", Method: http.MethodPut, Path: "/cruises/{cruiseID}",
		Summary: "Update a cruise (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-cruise", Method: http.MethodDelete, Path: "/cruises/{cruiseID}",
		Summary: "Delete a cruise (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
	huma.Register(api, huma.Operation{
		OperationID: "generate-cruise-enroll-token", Method: http.MethodPost, Path: "/cruises/{cruiseID}/enroll-token",
		Summary: "Generate a cruise enrollment token (admin)", Tags: tag,
	}, h.generateEnrollToken)
	huma.Register(api, huma.Operation{
		OperationID: "clear-cruise-enroll-token", Method: http.MethodDelete, Path: "/cruises/{cruiseID}/enroll-token",
		Summary: "Clear a cruise enrollment token (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.clearEnrollToken)
	huma.Register(api, huma.Operation{
		OperationID: "list-cruise-trips", Method: http.MethodGet, Path: "/cruises/{cruiseID}/trips",
		Summary: "List a cruise's child trips", Tags: tag,
	}, h.listChildTrips)
	huma.Register(api, huma.Operation{
		OperationID: "list-cruise-voyages", Method: http.MethodGet, Path: "/cruises/{cruiseID}/voyages",
		Summary: "List a cruise's child voyages", Tags: tag,
	}, h.listChildVoyages)

	huma.Register(api, huma.Operation{
		OperationID: "list-cruise-enrollments", Method: http.MethodGet,
		Path:    "/cruises/{cruiseID}/enrollments",
		Summary: "List a cruise's enrollments (admin)", Tags: tag,
	}, h.listEnrollments)
	huma.Register(api, huma.Operation{
		OperationID: "update-cruise-enrollment-status", Method: http.MethodPut,
		Path:    "/cruises/{cruiseID}/enrollments/{enrollmentID}/status",
		Summary: "Update a cruise enrollment's status (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.updateEnrollmentStatus)
	huma.Register(api, huma.Operation{
		OperationID: "assign-cruise-enrollment-trip", Method: http.MethodPut,
		Path:    "/cruises/{cruiseID}/enrollments/{enrollmentID}/trip",
		Summary: "Assign a cruise enrollment to a trip (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.assignEnrollmentToTrip)
	huma.Register(api, huma.Operation{
		OperationID: "delete-cruise-enrollment", Method: http.MethodDelete,
		Path:    "/cruises/{cruiseID}/enrollments/{enrollmentID}",
		Summary: "Delete a cruise enrollment (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.deleteEnrollment)
}

func (h *CruiseHandler) list(ctx context.Context, in *pageParams) (*cruiseListOutput, error) {
	cruises, err := h.q.ListCruises(ctx, sqlcdb.ListCruisesParams{Limit: in.Limit, Offset: in.Offset})
	if err != nil {
		slog.Error("list cruises", "err", err)
		return nil, huma.Error500InternalServerError("failed to list cruises")
	}
	total, err := h.q.CountCruises(ctx)
	if err != nil {
		slog.Error("count cruises", "err", err)
		return nil, huma.Error500InternalServerError("failed to list cruises")
	}
	return &cruiseListOutput{Body: dto.NewPage(dto.CruisesFromDB(cruises), total, in.Limit, in.Offset)}, nil
}

func (h *CruiseHandler) get(ctx context.Context, in *cruiseIDParam) (*cruiseOutput, error) {
	cruise, err := h.q.GetCruise(ctx, in.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("cruise not found")
		}
		slog.Error("get cruise", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get cruise")
	}
	return &cruiseOutput{Body: dto.CruiseFromDB(cruise)}, nil
}

func (h *CruiseHandler) create(ctx context.Context, in *createCruiseInput) (*cruiseOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	user := middleware.GetUser(ctx)
	cruise, err := h.q.CreateCruise(ctx, sqlcdb.CreateCruiseParams{
		CreatedBy:     types.NullInt64{Int64: user.UserID, Valid: true},
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
		slog.Error("create cruise", "name", in.Body.Name, "err", err)
		return nil, huma.Error500InternalServerError("failed to create cruise")
	}
	return &cruiseOutput{Body: dto.CruiseFromDB(cruise)}, nil
}

func (h *CruiseHandler) update(ctx context.Context, in *updateCruiseInput) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
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
	}); err != nil {
		slog.Error("update cruise", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update cruise")
	}
	return &noContentOutput{}, nil
}

func (h *CruiseHandler) delete(ctx context.Context, in *cruiseIDParam) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.DeleteCruise(ctx, in.ID); err != nil {
		slog.Error("delete cruise", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete cruise")
	}
	return &noContentOutput{}, nil
}

func (h *CruiseHandler) generateEnrollToken(ctx context.Context, in *cruiseIDParam) (*tokenOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.verifyCruise(ctx, in.ID); err != nil {
		return nil, err
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, huma.Error500InternalServerError("failed to generate token")
	}
	token := hex.EncodeToString(b)
	if err := h.q.SetCruiseEnrollToken(ctx, sqlcdb.SetCruiseEnrollTokenParams{
		EnrollToken: types.NullString{String: token, Valid: true},
		ID:          in.ID,
	}); err != nil {
		slog.Error("set cruise enroll token", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to set token")
	}
	out := &tokenOutput{}
	out.Body.Token = token
	return out, nil
}

func (h *CruiseHandler) clearEnrollToken(ctx context.Context, in *cruiseIDParam) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.ClearCruiseEnrollToken(ctx, in.ID); err != nil {
		slog.Error("clear cruise enroll token", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to clear token")
	}
	return &noContentOutput{}, nil
}

func (h *CruiseHandler) listChildTrips(ctx context.Context, in *cruiseIDParam) (*cruiseTripsOutput, error) {
	if err := h.verifyCruise(ctx, in.ID); err != nil {
		return nil, err
	}
	trips, err := h.q.ListCruiseTrips(ctx, types.NullInt64{Int64: in.ID, Valid: true})
	if err != nil {
		slog.Error("list cruise child trips", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list trips")
	}
	return &cruiseTripsOutput{Body: dto.TripsFromDB(trips)}, nil
}

func (h *CruiseHandler) listChildVoyages(ctx context.Context, in *cruiseIDParam) (*cruiseVoyagesOutput, error) {
	if err := h.verifyCruise(ctx, in.ID); err != nil {
		return nil, err
	}
	voyages, err := h.q.ListCruiseVoyages(ctx, types.NullInt64{Int64: in.ID, Valid: true})
	if err != nil {
		slog.Error("list cruise child voyages", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list voyages")
	}
	return &cruiseVoyagesOutput{Body: dto.VoyagesFromDB(voyages)}, nil
}

func (h *CruiseHandler) listEnrollments(ctx context.Context, in *cruiseEnrollmentsParam) (*cruiseEnrollmentsOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	enrollments, err := h.q.ListCruiseEnrollments(ctx, in.CruiseID)
	if err != nil {
		slog.Error("list cruise enrollments", "cruise_id", in.CruiseID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list enrollments")
	}
	return &cruiseEnrollmentsOutput{Body: dto.CruiseEnrollmentsFromDB(enrollments)}, nil
}

func (h *CruiseHandler) updateEnrollmentStatus(ctx context.Context, in *cruiseEnrollmentStatusInput) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.UpdateCruiseEnrollmentStatus(ctx, sqlcdb.UpdateCruiseEnrollmentStatusParams{
		Status: in.Body.Status,
		ID:     in.EnrollmentID,
	}); err != nil {
		slog.Error("update cruise enrollment status", "enrollment_id", in.EnrollmentID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update status")
	}
	return &noContentOutput{}, nil
}

func (h *CruiseHandler) assignEnrollmentToTrip(ctx context.Context, in *cruiseEnrollmentTripInput) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.AssignCruiseEnrollmentToTrip(ctx, sqlcdb.AssignCruiseEnrollmentToTripParams{
		TripID: nullInt64(in.Body.TripID),
		ID:     in.EnrollmentID,
	}); err != nil {
		slog.Error("assign cruise enrollment to trip", "enrollment_id", in.EnrollmentID, "err", err)
		return nil, huma.Error500InternalServerError("failed to assign enrollment")
	}
	return &noContentOutput{}, nil
}

func (h *CruiseHandler) deleteEnrollment(ctx context.Context, in *cruiseEnrollmentParam) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.DeleteCruiseEnrollment(ctx, in.EnrollmentID); err != nil {
		slog.Error("delete cruise enrollment", "enrollment_id", in.EnrollmentID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete enrollment")
	}
	return &noContentOutput{}, nil
}

// verifyCruise confirms the cruise exists.
func (h *CruiseHandler) verifyCruise(ctx context.Context, cruiseID int64) error {
	if _, err := h.q.GetCruise(ctx, cruiseID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return huma.Error404NotFound("cruise not found")
		}
		slog.Error("verify cruise", "cruise_id", cruiseID, "err", err)
		return huma.Error500InternalServerError("failed to verify cruise")
	}
	return nil
}
