package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type CruiseEnrollmentHandler struct {
	q sqlcdb.Querier
}

func NewCruiseEnrollmentHandler(q sqlcdb.Querier) *CruiseEnrollmentHandler {
	return &CruiseEnrollmentHandler{q: q}
}

type cruiseEnrollmentsParam struct {
	Slug     string `path:"slug" doc:"Organization slug"`
	CruiseID int64  `path:"cruiseID" doc:"Cruise ID"`
}

type cruiseEnrollmentParam struct {
	Slug         string `path:"slug" doc:"Organization slug"`
	CruiseID     int64  `path:"cruiseID" doc:"Cruise ID"`
	EnrollmentID int64  `path:"enrollmentID" doc:"Enrollment ID"`
}

type cruiseEnrollmentStatusInput struct {
	Slug         string `path:"slug" doc:"Organization slug"`
	CruiseID     int64  `path:"cruiseID" doc:"Cruise ID"`
	EnrollmentID int64  `path:"enrollmentID" doc:"Enrollment ID"`
	Body         dto.EnrollmentStatusBody
}

type cruiseEnrollmentTripInput struct {
	Slug         string `path:"slug" doc:"Organization slug"`
	CruiseID     int64  `path:"cruiseID" doc:"Cruise ID"`
	EnrollmentID int64  `path:"enrollmentID" doc:"Enrollment ID"`
	Body         dto.AssignTripBody
}

type cruiseEnrollmentsOutput struct {
	Body []dto.CruiseEnrollmentDetail
}

// RegisterCruiseEnrollmentRoutes wires the cruise enrollment management routes.
func RegisterCruiseEnrollmentRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewCruiseEnrollmentHandler(q)
	tag := []string{"Cruise enrollments"}

	huma.Register(api, huma.Operation{
		OperationID: "list-cruise-enrollments", Method: http.MethodGet,
		Path:    "/orgs/{slug}/cruises/{cruiseID}/enrollments",
		Summary: "List a cruise's enrollments", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "update-cruise-enrollment-status", Method: http.MethodPut,
		Path:    "/orgs/{slug}/cruises/{cruiseID}/enrollments/{enrollmentID}/status",
		Summary: "Update a cruise enrollment's status (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.updateStatus)
	huma.Register(api, huma.Operation{
		OperationID: "assign-cruise-enrollment-trip", Method: http.MethodPut,
		Path:    "/orgs/{slug}/cruises/{cruiseID}/enrollments/{enrollmentID}/trip",
		Summary: "Assign a cruise enrollment to a trip (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.assignToTrip)
	huma.Register(api, huma.Operation{
		OperationID: "delete-cruise-enrollment", Method: http.MethodDelete,
		Path:    "/orgs/{slug}/cruises/{cruiseID}/enrollments/{enrollmentID}",
		Summary: "Delete a cruise enrollment (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
}

func (h *CruiseEnrollmentHandler) list(ctx context.Context, in *cruiseEnrollmentsParam) (*cruiseEnrollmentsOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	enrollments, err := h.q.ListCruiseEnrollments(ctx, sqlcdb.ListCruiseEnrollmentsParams{
		CruiseID: in.CruiseID,
		OrgID:    octx.OrgID,
	})
	if err != nil {
		slog.Error("list cruise enrollments", "cruise_id", in.CruiseID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list enrollments")
	}
	return &cruiseEnrollmentsOutput{Body: dto.CruiseEnrollmentsFromDB(enrollments)}, nil
}

func (h *CruiseEnrollmentHandler) updateStatus(ctx context.Context, in *cruiseEnrollmentStatusInput) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.UpdateCruiseEnrollmentStatus(ctx, sqlcdb.UpdateCruiseEnrollmentStatusParams{
		Status: in.Body.Status,
		ID:     in.EnrollmentID,
		OrgID:  octx.OrgID,
	}); err != nil {
		slog.Error("update cruise enrollment status", "enrollment_id", in.EnrollmentID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update status")
	}
	return &noContentOutput{}, nil
}

func (h *CruiseEnrollmentHandler) assignToTrip(ctx context.Context, in *cruiseEnrollmentTripInput) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.AssignCruiseEnrollmentToTrip(ctx, sqlcdb.AssignCruiseEnrollmentToTripParams{
		TripID: nullInt64(in.Body.TripID),
		ID:     in.EnrollmentID,
		OrgID:  octx.OrgID,
	}); err != nil {
		slog.Error("assign cruise enrollment to trip", "enrollment_id", in.EnrollmentID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to assign enrollment")
	}
	return &noContentOutput{}, nil
}

func (h *CruiseEnrollmentHandler) delete(ctx context.Context, in *cruiseEnrollmentParam) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.DeleteCruiseEnrollment(ctx, sqlcdb.DeleteCruiseEnrollmentParams{
		ID:    in.EnrollmentID,
		OrgID: octx.OrgID,
	}); err != nil {
		slog.Error("delete cruise enrollment", "enrollment_id", in.EnrollmentID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete enrollment")
	}
	return &noContentOutput{}, nil
}
