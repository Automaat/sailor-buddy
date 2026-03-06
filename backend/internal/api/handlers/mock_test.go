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
	getDashboardStatsFn                  func(ctx context.Context, ownerID int64) (sqlcdb.GetDashboardStatsRow, error)
	getCruisesByYearFn                   func(ctx context.Context, ownerID int64) ([]sqlcdb.GetCruisesByYearRow, error)
	getYachtByNameFn                     func(ctx context.Context, arg sqlcdb.GetYachtByNameParams) (sqlcdb.Yacht, error)
	getCrewMemberByNameFn                func(ctx context.Context, arg sqlcdb.GetCrewMemberByNameParams) (sqlcdb.CrewMember, error)
	getCrewAssignmentByCruiseAndMemberFn func(ctx context.Context, arg sqlcdb.GetCrewAssignmentByCruiseAndMemberParams) (sqlcdb.GetCrewAssignmentByCruiseAndMemberRow, error)
	upsertVoyageOpinionFn                func(ctx context.Context, arg sqlcdb.UpsertVoyageOpinionParams) (sqlcdb.VoyageOpinion, error)
	listCruiseVoyageOpinionsFn           func(ctx context.Context, cruiseID int64) ([]sqlcdb.ListCruiseVoyageOpinionsRow, error)
	getVoyageOpinionFn                   func(ctx context.Context, id int64) (sqlcdb.VoyageOpinion, error)
	deleteVoyageOpinionFn                func(ctx context.Context, id int64) error
}

func (m *mockQuerier) ListCruises(ctx context.Context, ownerID int64) ([]sqlcdb.Cruise, error) {
	if m.listCruisesFn == nil {
		panic("unexpected call to ListCruises: listCruisesFn is nil")
	}
	return m.listCruisesFn(ctx, ownerID)
}

func (m *mockQuerier) GetCruise(ctx context.Context, arg sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
	if m.getCruiseFn == nil {
		panic("unexpected call to GetCruise: getCruiseFn is nil")
	}
	return m.getCruiseFn(ctx, arg)
}

func (m *mockQuerier) CreateCruise(ctx context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
	if m.createCruiseFn == nil {
		panic("unexpected call to CreateCruise: createCruiseFn is nil")
	}
	return m.createCruiseFn(ctx, arg)
}

func (m *mockQuerier) UpdateCruise(ctx context.Context, arg sqlcdb.UpdateCruiseParams) error {
	if m.updateCruiseFn == nil {
		panic("unexpected call to UpdateCruise: updateCruiseFn is nil")
	}
	return m.updateCruiseFn(ctx, arg)
}

func (m *mockQuerier) DeleteCruise(ctx context.Context, arg sqlcdb.DeleteCruiseParams) error {
	if m.deleteCruiseFn == nil {
		panic("unexpected call to DeleteCruise: deleteCruiseFn is nil")
	}
	return m.deleteCruiseFn(ctx, arg)
}

func (m *mockQuerier) ListYachts(ctx context.Context, ownerID int64) ([]sqlcdb.Yacht, error) {
	if m.listYachtsFn == nil {
		panic("unexpected call to ListYachts: listYachtsFn is nil")
	}
	return m.listYachtsFn(ctx, ownerID)
}

func (m *mockQuerier) GetYacht(ctx context.Context, arg sqlcdb.GetYachtParams) (sqlcdb.Yacht, error) {
	if m.getYachtFn == nil {
		panic("unexpected call to GetYacht: getYachtFn is nil")
	}
	return m.getYachtFn(ctx, arg)
}

func (m *mockQuerier) CreateYacht(ctx context.Context, arg sqlcdb.CreateYachtParams) (sqlcdb.Yacht, error) {
	if m.createYachtFn == nil {
		panic("unexpected call to CreateYacht: createYachtFn is nil")
	}
	return m.createYachtFn(ctx, arg)
}

func (m *mockQuerier) UpdateYacht(ctx context.Context, arg sqlcdb.UpdateYachtParams) error {
	if m.updateYachtFn == nil {
		panic("unexpected call to UpdateYacht: updateYachtFn is nil")
	}
	return m.updateYachtFn(ctx, arg)
}

func (m *mockQuerier) DeleteYacht(ctx context.Context, arg sqlcdb.DeleteYachtParams) error {
	if m.deleteYachtFn == nil {
		panic("unexpected call to DeleteYacht: deleteYachtFn is nil")
	}
	return m.deleteYachtFn(ctx, arg)
}

func (m *mockQuerier) ListTrainings(ctx context.Context, userID int64) ([]sqlcdb.Training, error) {
	if m.listTrainingsFn == nil {
		panic("unexpected call to ListTrainings: listTrainingsFn is nil")
	}
	return m.listTrainingsFn(ctx, userID)
}

func (m *mockQuerier) GetTraining(ctx context.Context, arg sqlcdb.GetTrainingParams) (sqlcdb.Training, error) {
	if m.getTrainingFn == nil {
		panic("unexpected call to GetTraining: getTrainingFn is nil")
	}
	return m.getTrainingFn(ctx, arg)
}

func (m *mockQuerier) CreateTraining(ctx context.Context, arg sqlcdb.CreateTrainingParams) (sqlcdb.Training, error) {
	if m.createTrainingFn == nil {
		panic("unexpected call to CreateTraining: createTrainingFn is nil")
	}
	return m.createTrainingFn(ctx, arg)
}

func (m *mockQuerier) UpdateTraining(ctx context.Context, arg sqlcdb.UpdateTrainingParams) error {
	if m.updateTrainingFn == nil {
		panic("unexpected call to UpdateTraining: updateTrainingFn is nil")
	}
	return m.updateTrainingFn(ctx, arg)
}

func (m *mockQuerier) DeleteTraining(ctx context.Context, arg sqlcdb.DeleteTrainingParams) error {
	if m.deleteTrainingFn == nil {
		panic("unexpected call to DeleteTraining: deleteTrainingFn is nil")
	}
	return m.deleteTrainingFn(ctx, arg)
}

func (m *mockQuerier) ListCrewMembers(ctx context.Context, ownerID int64) ([]sqlcdb.CrewMember, error) {
	if m.listCrewMembersFn == nil {
		panic("unexpected call to ListCrewMembers: listCrewMembersFn is nil")
	}
	return m.listCrewMembersFn(ctx, ownerID)
}

func (m *mockQuerier) GetCrewMember(ctx context.Context, arg sqlcdb.GetCrewMemberParams) (sqlcdb.CrewMember, error) {
	if m.getCrewMemberFn == nil {
		panic("unexpected call to GetCrewMember: getCrewMemberFn is nil")
	}
	return m.getCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) CreateCrewMember(ctx context.Context, arg sqlcdb.CreateCrewMemberParams) (sqlcdb.CrewMember, error) {
	if m.createCrewMemberFn == nil {
		panic("unexpected call to CreateCrewMember: createCrewMemberFn is nil")
	}
	return m.createCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) UpdateCrewMember(ctx context.Context, arg sqlcdb.UpdateCrewMemberParams) error {
	if m.updateCrewMemberFn == nil {
		panic("unexpected call to UpdateCrewMember: updateCrewMemberFn is nil")
	}
	return m.updateCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) DeleteCrewMember(ctx context.Context, arg sqlcdb.DeleteCrewMemberParams) error {
	if m.deleteCrewMemberFn == nil {
		panic("unexpected call to DeleteCrewMember: deleteCrewMemberFn is nil")
	}
	return m.deleteCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) CreateCrewAssignment(ctx context.Context, arg sqlcdb.CreateCrewAssignmentParams) (sqlcdb.CrewAssignment, error) {
	if m.createCrewAssignmentFn == nil {
		panic("unexpected call to CreateCrewAssignment: createCrewAssignmentFn is nil")
	}
	return m.createCrewAssignmentFn(ctx, arg)
}

func (m *mockQuerier) ListCruiseCrewAssignments(ctx context.Context, arg sqlcdb.ListCruiseCrewAssignmentsParams) ([]sqlcdb.ListCruiseCrewAssignmentsRow, error) {
	if m.listCruiseCrewFn == nil {
		panic("unexpected call to ListCruiseCrewAssignments: listCruiseCrewFn is nil")
	}
	return m.listCruiseCrewFn(ctx, arg)
}

func (m *mockQuerier) DeleteCrewAssignment(ctx context.Context, arg sqlcdb.DeleteCrewAssignmentParams) error {
	if m.deleteCrewAssignmentFn == nil {
		panic("unexpected call to DeleteCrewAssignment: deleteCrewAssignmentFn is nil")
	}
	return m.deleteCrewAssignmentFn(ctx, arg)
}

func (m *mockQuerier) GetDashboardStats(ctx context.Context, ownerID int64) (sqlcdb.GetDashboardStatsRow, error) {
	if m.getDashboardStatsFn == nil {
		panic("unexpected call to GetDashboardStats: getDashboardStatsFn is nil")
	}
	return m.getDashboardStatsFn(ctx, ownerID)
}

func (m *mockQuerier) GetCruisesByYear(ctx context.Context, ownerID int64) ([]sqlcdb.GetCruisesByYearRow, error) {
	if m.getCruisesByYearFn == nil {
		panic("unexpected call to GetCruisesByYear: getCruisesByYearFn is nil")
	}
	return m.getCruisesByYearFn(ctx, ownerID)
}

func (m *mockQuerier) GetYachtByName(ctx context.Context, arg sqlcdb.GetYachtByNameParams) (sqlcdb.Yacht, error) {
	if m.getYachtByNameFn == nil {
		panic("unexpected call to GetYachtByName: getYachtByNameFn is nil")
	}
	return m.getYachtByNameFn(ctx, arg)
}

func (m *mockQuerier) GetCrewMemberByName(ctx context.Context, arg sqlcdb.GetCrewMemberByNameParams) (sqlcdb.CrewMember, error) {
	if m.getCrewMemberByNameFn == nil {
		panic("unexpected call to GetCrewMemberByName: getCrewMemberByNameFn is nil")
	}
	return m.getCrewMemberByNameFn(ctx, arg)
}

func (m *mockQuerier) CreateUser(context.Context, sqlcdb.CreateUserParams) (sqlcdb.User, error) {
	panic("unexpected call")
}

func (m *mockQuerier) CreateVoyageOpinion(context.Context, sqlcdb.CreateVoyageOpinionParams) (sqlcdb.VoyageOpinion, error) {
	panic("unexpected call")
}

func (m *mockQuerier) DeleteVoyageOpinion(ctx context.Context, id int64) error {
	if m.deleteVoyageOpinionFn != nil {
		return m.deleteVoyageOpinionFn(ctx, id)
	}
	panic("unexpected call to DeleteVoyageOpinion")
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

func (m *mockQuerier) GetVoyageOpinion(ctx context.Context, id int64) (sqlcdb.VoyageOpinion, error) {
	if m.getVoyageOpinionFn != nil {
		return m.getVoyageOpinionFn(ctx, id)
	}
	panic("unexpected call to GetVoyageOpinion")
}

func (m *mockQuerier) LinkFirebaseUIDByEmail(context.Context, sqlcdb.LinkFirebaseUIDByEmailParams) (sqlcdb.User, error) {
	panic("unexpected call")
}

func (m *mockQuerier) ListCruiseVoyageOpinions(ctx context.Context, cruiseID int64) ([]sqlcdb.ListCruiseVoyageOpinionsRow, error) {
	if m.listCruiseVoyageOpinionsFn != nil {
		return m.listCruiseVoyageOpinionsFn(ctx, cruiseID)
	}
	panic("unexpected call to ListCruiseVoyageOpinions")
}

func (m *mockQuerier) UpdateUser(context.Context, sqlcdb.UpdateUserParams) error {
	panic("unexpected call")
}

func (m *mockQuerier) UpsertUserByFirebaseUID(context.Context, sqlcdb.UpsertUserByFirebaseUIDParams) (sqlcdb.User, error) {
	panic("unexpected call")
}

func (m *mockQuerier) GetCrewAssignmentByCruiseAndMember(ctx context.Context, arg sqlcdb.GetCrewAssignmentByCruiseAndMemberParams) (sqlcdb.GetCrewAssignmentByCruiseAndMemberRow, error) {
	if m.getCrewAssignmentByCruiseAndMemberFn != nil {
		return m.getCrewAssignmentByCruiseAndMemberFn(ctx, arg)
	}
	panic("unexpected call to GetCrewAssignmentByCruiseAndMember")
}

func (m *mockQuerier) UpsertVoyageOpinion(ctx context.Context, arg sqlcdb.UpsertVoyageOpinionParams) (sqlcdb.VoyageOpinion, error) {
	if m.upsertVoyageOpinionFn != nil {
		return m.upsertVoyageOpinionFn(ctx, arg)
	}
	panic("unexpected call to UpsertVoyageOpinion")
}

func userCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, middleware.UserCtxKey, &auth.Claims{
		UserID: 1, Email: "test@example.com", Name: "Test User",
	})
}
