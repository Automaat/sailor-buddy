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
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type EnrollmentHandler struct {
	q sqlcdb.Querier
}

func NewEnrollmentHandler(q sqlcdb.Querier) *EnrollmentHandler {
	return &EnrollmentHandler{q: q}
}

type tokenPathParam struct {
	Token string `path:"token" doc:"Enrollment share token"`
}

type enrollInput struct {
	Token string `path:"token" doc:"Enrollment share token"`
	Body  dto.EnrollBody
}

type enrollInfoOutput struct {
	Body dto.EnrollInfo
}

type enrollmentOutput struct {
	Body dto.Enrollment
}

type tripEnrollmentsOutput struct {
	Body []dto.TripEnrollmentDetail
}

type tokenOutput struct {
	Body struct {
		Token string `json:"token"`
	}
}

type tripEnrollmentStatusInput struct {
	TripID int64 `path:"tripID" doc:"Trip ID"`
	ID     int64 `path:"id" doc:"Enrollment ID"`
	Body   dto.EnrollmentStatusBody
}

type tripEnrollmentParam struct {
	TripID int64 `path:"tripID" doc:"Trip ID"`
	ID     int64 `path:"id" doc:"Enrollment ID"`
}

// RegisterEnrollmentRoutes wires the share-token enrollment flow and the
// owner-side trip enrollment management onto the API.
func RegisterEnrollmentRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewEnrollmentHandler(q)
	tag := []string{"Enrollment"}

	huma.Register(api, huma.Operation{
		OperationID: "resolve-enroll-token", Method: http.MethodGet, Path: "/enroll/{token}",
		Summary: "Resolve an enrollment token to its trip or cruise", Tags: tag,
	}, h.getByToken)
	huma.Register(api, huma.Operation{
		OperationID: "enroll", Method: http.MethodPost, Path: "/enroll/{token}",
		Summary: "Self-enroll via a share token", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.enroll)

	huma.Register(api, huma.Operation{
		OperationID: "generate-trip-enroll-token", Method: http.MethodPost, Path: "/trips/{tripID}/enroll-token",
		Summary: "Generate a trip enrollment share token", Tags: tag,
	}, h.generateToken)
	huma.Register(api, huma.Operation{
		OperationID: "clear-trip-enroll-token", Method: http.MethodDelete, Path: "/trips/{tripID}/enroll-token",
		Summary: "Clear a trip enrollment share token", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.clearToken)
	huma.Register(api, huma.Operation{
		OperationID: "list-trip-enrollments", Method: http.MethodGet, Path: "/trips/{tripID}/enrollments",
		Summary: "List a trip's enrollments", Tags: tag,
	}, h.listEnrollments)
	huma.Register(api, huma.Operation{
		OperationID: "update-trip-enrollment-status", Method: http.MethodPut, Path: "/trips/{tripID}/enrollments/{id}/status",
		Summary: "Update a trip enrollment's status", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.updateStatus)
	huma.Register(api, huma.Operation{
		OperationID: "delete-trip-enrollment", Method: http.MethodDelete, Path: "/trips/{tripID}/enrollments/{id}",
		Summary: "Delete a trip enrollment", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.deleteEnrollment)
}

// getByToken resolves a share token, which may belong to a trip or a cruise.
func (h *EnrollmentHandler) getByToken(ctx context.Context, in *tokenPathParam) (*enrollInfoOutput, error) {
	user := middleware.GetUser(ctx)
	tok := types.NullString{String: in.Token, Valid: true}

	trip, terr := h.q.GetTripByEnrollToken(ctx, tok)
	if terr == nil {
		counts, _ := h.q.CountTripEnrollments(ctx, trip.ID)
		info := dto.EnrollInfo{
			Kind:          "trip",
			Trip:          new(dto.EnrollTripFromRow(trip)),
			AcceptedCount: counts.Accepted,
			TotalCount:    counts.Total,
		}
		if e, err := h.q.GetUserTripEnrollment(ctx, sqlcdb.GetUserTripEnrollmentParams{TripID: trip.ID, UserID: user.UserID}); err == nil {
			info.Enrolled = true
			info.Enrollment = new(dto.TripEnrollmentToDTO(e))
		}
		return &enrollInfoOutput{Body: info}, nil
	}
	if !errors.Is(terr, sql.ErrNoRows) {
		slog.Error("get trip by enroll token", "err", terr)
		return nil, huma.Error500InternalServerError("failed to get trip")
	}

	cruise, cerr := h.q.GetCruiseByEnrollToken(ctx, tok)
	if cerr != nil {
		if errors.Is(cerr, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("invalid enrollment link")
		}
		slog.Error("get cruise by enroll token", "err", cerr)
		return nil, huma.Error500InternalServerError("failed to get cruise")
	}
	counts, _ := h.q.CountCruiseEnrollments(ctx, cruise.ID)
	childTrips, tlistErr := h.q.ListCruiseTrips(ctx, types.NullInt64{Int64: cruise.ID, Valid: true})
	if tlistErr != nil {
		slog.Error("list cruise trips", "cruise_id", cruise.ID, "err", tlistErr)
		childTrips = nil
	}
	info := dto.EnrollInfo{
		Kind:          "cruise",
		Cruise:        new(dto.EnrollCruiseFromRow(cruise)),
		Trips:         dto.TripsFromDB(childTrips),
		AcceptedCount: counts.Accepted,
		TotalCount:    counts.Total,
	}
	if e, err := h.q.GetUserCruiseEnrollment(ctx, sqlcdb.GetUserCruiseEnrollmentParams{CruiseID: cruise.ID, UserID: user.UserID}); err == nil {
		info.Enrolled = true
		info.Enrollment = new(dto.CruiseEnrollmentToDTO(e))
	}
	return &enrollInfoOutput{Body: info}, nil
}

// enroll self-enrolls the caller into the trip or cruise the token resolves to.
func (h *EnrollmentHandler) enroll(ctx context.Context, in *enrollInput) (*enrollmentOutput, error) {
	user := middleware.GetUser(ctx)
	tok := types.NullString{String: in.Token, Valid: true}

	trip, terr := h.q.GetTripByEnrollToken(ctx, tok)
	if terr == nil {
		status, _ := h.q.GetTripStatus(ctx, trip.ID)
		if status != sqlcdb.TripStatusPlanned {
			return nil, huma.Error409Conflict("enrollment closed: trip is not planned")
		}
		e, err := h.q.CreateTripEnrollment(ctx, sqlcdb.CreateTripEnrollmentParams{
			TripID: trip.ID,
			UserID: user.UserID,
			Note:   nullString(in.Body.Note),
		})
		if err != nil {
			if isUniqueViolation(err) {
				return nil, huma.Error409Conflict("already enrolled")
			}
			slog.Error("create trip enrollment", "trip_id", trip.ID, "user_id", user.UserID, "err", err)
			return nil, huma.Error500InternalServerError("failed to enroll")
		}
		return &enrollmentOutput{Body: dto.TripEnrollmentToDTO(e)}, nil
	}
	if !errors.Is(terr, sql.ErrNoRows) {
		slog.Error("get trip by enroll token for enroll", "err", terr)
		return nil, huma.Error500InternalServerError("failed to get trip")
	}

	cruise, cerr := h.q.GetCruiseByEnrollToken(ctx, tok)
	if cerr != nil {
		if errors.Is(cerr, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("invalid enrollment link")
		}
		slog.Error("get cruise by enroll token for enroll", "err", cerr)
		return nil, huma.Error500InternalServerError("failed to get cruise")
	}
	e, err := h.q.CreateCruiseEnrollment(ctx, sqlcdb.CreateCruiseEnrollmentParams{
		CruiseID: cruise.ID,
		UserID:   user.UserID,
		Note:     nullString(in.Body.Note),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, huma.Error409Conflict("already enrolled")
		}
		slog.Error("create cruise enrollment", "cruise_id", cruise.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to enroll")
	}
	return &enrollmentOutput{Body: dto.CruiseEnrollmentToDTO(e)}, nil
}

func (h *EnrollmentHandler) generateToken(ctx context.Context, in *tripIDParam) (*tokenOutput, error) {
	user := middleware.GetUser(ctx)
	if _, err := h.q.GetTrip(ctx, sqlcdb.GetTripParams{ID: in.ID, OwnerID: user.UserID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found")
		}
		slog.Error("get trip for token generation", "trip_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get trip")
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, huma.Error500InternalServerError("failed to generate token")
	}
	token := hex.EncodeToString(b)

	if err := h.q.SetTripEnrollToken(ctx, sqlcdb.SetTripEnrollTokenParams{
		EnrollToken: types.NullString{String: token, Valid: true},
		ID:          in.ID,
		OwnerID:     user.UserID,
	}); err != nil {
		slog.Error("set trip enroll token", "trip_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to set token")
	}
	out := &tokenOutput{}
	out.Body.Token = token
	return out, nil
}

func (h *EnrollmentHandler) clearToken(ctx context.Context, in *tripIDParam) (*noContentOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.q.ClearTripEnrollToken(ctx, sqlcdb.ClearTripEnrollTokenParams{ID: in.ID, OwnerID: user.UserID}); err != nil {
		slog.Error("clear trip enroll token", "trip_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to clear token")
	}
	return &noContentOutput{}, nil
}

func (h *EnrollmentHandler) listEnrollments(ctx context.Context, in *tripIDParam) (*tripEnrollmentsOutput, error) {
	user := middleware.GetUser(ctx)
	enrollments, err := h.q.ListTripEnrollments(ctx, sqlcdb.ListTripEnrollmentsParams{TripID: in.ID, OwnerID: user.UserID})
	if err != nil {
		slog.Error("list trip enrollments", "trip_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list enrollments")
	}
	return &tripEnrollmentsOutput{Body: dto.TripEnrollmentsFromDB(enrollments)}, nil
}

func (h *EnrollmentHandler) updateStatus(ctx context.Context, in *tripEnrollmentStatusInput) (*noContentOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.q.UpdateTripEnrollmentStatus(ctx, sqlcdb.UpdateTripEnrollmentStatusParams{
		Status:  in.Body.Status,
		ID:      in.ID,
		OwnerID: user.UserID,
	}); err != nil {
		slog.Error("update trip enrollment status", "enrollment_id", in.ID, "user_id", user.UserID, "status", in.Body.Status, "err", err)
		return nil, huma.Error500InternalServerError("failed to update status")
	}
	return &noContentOutput{}, nil
}

func (h *EnrollmentHandler) deleteEnrollment(ctx context.Context, in *tripEnrollmentParam) (*noContentOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.q.DeleteTripEnrollment(ctx, sqlcdb.DeleteTripEnrollmentParams{ID: in.ID, OwnerID: user.UserID}); err != nil {
		slog.Error("delete trip enrollment", "enrollment_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete enrollment")
	}
	return &noContentOutput{}, nil
}

// isUniqueViolation reports whether err is a PostgreSQL unique-constraint error.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
