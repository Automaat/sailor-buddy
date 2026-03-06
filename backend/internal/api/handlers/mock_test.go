package handlers

import (
	"context"
	"database/sql"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type mockQuerier struct {
	listCruisesFn          func(ctx context.Context, ownerID int64) ([]sqlcdb.Cruise, error)
	getCruiseFn            func(ctx context.Context, arg sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error)
	createCruiseFn         func(ctx context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error)
	updateCruiseFn         func(ctx context.Context, arg sqlcdb.UpdateCruiseParams) error
	deleteCruiseFn         func(ctx context.Context, arg sqlcdb.DeleteCruiseParams) error
	listYachtsFn           func(ctx context.Context, ownerID int64) ([]sqlcdb.Yacht, error)
	getYachtFn             func(ctx context.Context, arg sqlcdb.GetYachtParams) (sqlcdb.Yacht, error)
	createYachtFn          func(ctx context.Context, arg sqlcdb.CreateYachtParams) (sqlcdb.Yacht, error)
	updateYachtFn          func(ctx context.Context, arg sqlcdb.UpdateYachtParams) error
	deleteYachtFn          func(ctx context.Context, arg sqlcdb.DeleteYachtParams) error
	listTrainingsFn        func(ctx context.Context, userID int64) ([]sqlcdb.Training, error)
	getTrainingFn          func(ctx context.Context, arg sqlcdb.GetTrainingParams) (sqlcdb.Training, error)
	createTrainingFn       func(ctx context.Context, arg sqlcdb.CreateTrainingParams) (sqlcdb.Training, error)
	updateTrainingFn       func(ctx context.Context, arg sqlcdb.UpdateTrainingParams) error
	deleteTrainingFn       func(ctx context.Context, arg sqlcdb.DeleteTrainingParams) error
	listCrewMembersFn      func(ctx context.Context, ownerID int64) ([]sqlcdb.CrewMember, error)
	getCrewMemberFn        func(ctx context.Context, arg sqlcdb.GetCrewMemberParams) (sqlcdb.CrewMember, error)
	createCrewMemberFn     func(ctx context.Context, arg sqlcdb.CreateCrewMemberParams) (sqlcdb.CrewMember, error)
	updateCrewMemberFn     func(ctx context.Context, arg sqlcdb.UpdateCrewMemberParams) error
	deleteCrewMemberFn     func(ctx context.Context, arg sqlcdb.DeleteCrewMemberParams) error
	createCrewAssignmentFn func(ctx context.Context, arg sqlcdb.CreateCrewAssignmentParams) (sqlcdb.CrewAssignment, error)
	listCruiseCrewFn       func(ctx context.Context, arg sqlcdb.ListCruiseCrewAssignmentsParams) ([]sqlcdb.ListCruiseCrewAssignmentsRow, error)
	deleteCrewAssignmentFn func(ctx context.Context, arg sqlcdb.DeleteCrewAssignmentParams) error
	getDashboardStatsFn    func(ctx context.Context, ownerID int64) (sqlcdb.GetDashboardStatsRow, error)
	getCruisesByYearFn     func(ctx context.Context, ownerID int64) ([]sqlcdb.GetCruisesByYearRow, error)
	getYachtByNameFn       func(ctx context.Context, arg sqlcdb.GetYachtByNameParams) (sqlcdb.Yacht, error)
	getCrewMemberByNameFn  func(ctx context.Context, arg sqlcdb.GetCrewMemberByNameParams) (sqlcdb.CrewMember, error)
}

func (m *mockQuerier) ListCruises(ctx context.Context, ownerID int64) ([]sqlcdb.Cruise, error) {
	return m.listCruisesFn(ctx, ownerID)
}

func (m *mockQuerier) GetCruise(ctx context.Context, arg sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
	return m.getCruiseFn(ctx, arg)
}

func (m *mockQuerier) CreateCruise(ctx context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
	return m.createCruiseFn(ctx, arg)
}

func (m *mockQuerier) UpdateCruise(ctx context.Context, arg sqlcdb.UpdateCruiseParams) error {
	return m.updateCruiseFn(ctx, arg)
}

func (m *mockQuerier) DeleteCruise(ctx context.Context, arg sqlcdb.DeleteCruiseParams) error {
	return m.deleteCruiseFn(ctx, arg)
}

func (m *mockQuerier) ListYachts(ctx context.Context, ownerID int64) ([]sqlcdb.Yacht, error) {
	return m.listYachtsFn(ctx, ownerID)
}

func (m *mockQuerier) GetYacht(ctx context.Context, arg sqlcdb.GetYachtParams) (sqlcdb.Yacht, error) {
	return m.getYachtFn(ctx, arg)
}

func (m *mockQuerier) CreateYacht(ctx context.Context, arg sqlcdb.CreateYachtParams) (sqlcdb.Yacht, error) {
	return m.createYachtFn(ctx, arg)
}

func (m *mockQuerier) UpdateYacht(ctx context.Context, arg sqlcdb.UpdateYachtParams) error {
	return m.updateYachtFn(ctx, arg)
}

func (m *mockQuerier) DeleteYacht(ctx context.Context, arg sqlcdb.DeleteYachtParams) error {
	return m.deleteYachtFn(ctx, arg)
}

func (m *mockQuerier) ListTrainings(ctx context.Context, userID int64) ([]sqlcdb.Training, error) {
	return m.listTrainingsFn(ctx, userID)
}

func (m *mockQuerier) GetTraining(ctx context.Context, arg sqlcdb.GetTrainingParams) (sqlcdb.Training, error) {
	return m.getTrainingFn(ctx, arg)
}

func (m *mockQuerier) CreateTraining(ctx context.Context, arg sqlcdb.CreateTrainingParams) (sqlcdb.Training, error) {
	return m.createTrainingFn(ctx, arg)
}

func (m *mockQuerier) UpdateTraining(ctx context.Context, arg sqlcdb.UpdateTrainingParams) error {
	return m.updateTrainingFn(ctx, arg)
}

func (m *mockQuerier) DeleteTraining(ctx context.Context, arg sqlcdb.DeleteTrainingParams) error {
	return m.deleteTrainingFn(ctx, arg)
}

func (m *mockQuerier) ListCrewMembers(ctx context.Context, ownerID int64) ([]sqlcdb.CrewMember, error) {
	return m.listCrewMembersFn(ctx, ownerID)
}

func (m *mockQuerier) GetCrewMember(ctx context.Context, arg sqlcdb.GetCrewMemberParams) (sqlcdb.CrewMember, error) {
	return m.getCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) CreateCrewMember(ctx context.Context, arg sqlcdb.CreateCrewMemberParams) (sqlcdb.CrewMember, error) {
	return m.createCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) UpdateCrewMember(ctx context.Context, arg sqlcdb.UpdateCrewMemberParams) error {
	return m.updateCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) DeleteCrewMember(ctx context.Context, arg sqlcdb.DeleteCrewMemberParams) error {
	return m.deleteCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) CreateCrewAssignment(ctx context.Context, arg sqlcdb.CreateCrewAssignmentParams) (sqlcdb.CrewAssignment, error) {
	return m.createCrewAssignmentFn(ctx, arg)
}

func (m *mockQuerier) ListCruiseCrewAssignments(ctx context.Context, arg sqlcdb.ListCruiseCrewAssignmentsParams) ([]sqlcdb.ListCruiseCrewAssignmentsRow, error) {
	return m.listCruiseCrewFn(ctx, arg)
}

func (m *mockQuerier) DeleteCrewAssignment(ctx context.Context, arg sqlcdb.DeleteCrewAssignmentParams) error {
	return m.deleteCrewAssignmentFn(ctx, arg)
}

func (m *mockQuerier) GetDashboardStats(ctx context.Context, ownerID int64) (sqlcdb.GetDashboardStatsRow, error) {
	return m.getDashboardStatsFn(ctx, ownerID)
}

func (m *mockQuerier) GetCruisesByYear(ctx context.Context, ownerID int64) ([]sqlcdb.GetCruisesByYearRow, error) {
	return m.getCruisesByYearFn(ctx, ownerID)
}

func (m *mockQuerier) GetYachtByName(ctx context.Context, arg sqlcdb.GetYachtByNameParams) (sqlcdb.Yacht, error) {
	return m.getYachtByNameFn(ctx, arg)
}

func (m *mockQuerier) GetCrewMemberByName(ctx context.Context, arg sqlcdb.GetCrewMemberByNameParams) (sqlcdb.CrewMember, error) {
	return m.getCrewMemberByNameFn(ctx, arg)
}

func (m *mockQuerier) CreateUser(context.Context, sqlcdb.CreateUserParams) (sqlcdb.User, error) {
	panic("unexpected call")
}

func (m *mockQuerier) CreateVoyageOpinion(context.Context, sqlcdb.CreateVoyageOpinionParams) (sqlcdb.VoyageOpinion, error) {
	panic("unexpected call")
}

func (m *mockQuerier) DeleteVoyageOpinion(context.Context, int64) error {
	panic("unexpected call")
}

func (m *mockQuerier) GetCrewMemberCruises(context.Context, int64) ([]sqlcdb.GetCrewMemberCruisesRow, error) {
	panic("unexpected call")
}

func (m *mockQuerier) GetCrewMemberStats(context.Context, int64) (sqlcdb.GetCrewMemberStatsRow, error) {
	panic("unexpected call")
}

func (m *mockQuerier) GetUserByEmail(context.Context, string) (sqlcdb.User, error) {
	panic("unexpected call")
}

func (m *mockQuerier) GetUserByFirebaseUID(context.Context, sql.NullString) (sqlcdb.User, error) {
	panic("unexpected call")
}

func (m *mockQuerier) GetUserByID(context.Context, int64) (sqlcdb.User, error) {
	panic("unexpected call")
}

func (m *mockQuerier) GetVoyageOpinion(context.Context, int64) (sqlcdb.VoyageOpinion, error) {
	panic("unexpected call")
}

func (m *mockQuerier) LinkFirebaseUIDByEmail(context.Context, sqlcdb.LinkFirebaseUIDByEmailParams) (sqlcdb.User, error) {
	panic("unexpected call")
}

func (m *mockQuerier) ListCruiseVoyageOpinions(context.Context, int64) ([]sqlcdb.ListCruiseVoyageOpinionsRow, error) {
	panic("unexpected call")
}

func (m *mockQuerier) UpdateUser(context.Context, sqlcdb.UpdateUserParams) error {
	panic("unexpected call")
}

func (m *mockQuerier) UpsertUserByFirebaseUID(context.Context, sqlcdb.UpsertUserByFirebaseUIDParams) (sqlcdb.User, error) {
	panic("unexpected call")
}

func userCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, middleware.UserCtxKey, &auth.Claims{
		UserID: 1, Email: "test@example.com", Name: "Test User",
	})
}
