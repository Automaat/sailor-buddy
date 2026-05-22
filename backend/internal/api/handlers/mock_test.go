package handlers

import (
	"context"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// mockQuerier implements sqlcdb.Querier with overridable per-method funcs.
// Any method without a configured fn panics, surfacing unexpected calls.
type mockQuerier struct {
	// trips
	listTripsFn            func(ctx context.Context, arg sqlcdb.ListTripsParams) ([]sqlcdb.Trip, error)
	countTripsFn           func(ctx context.Context) (int64, error)
	getTripFn              func(ctx context.Context, id int64) (sqlcdb.Trip, error)
	createTripFn           func(ctx context.Context, arg sqlcdb.CreateTripParams) (sqlcdb.Trip, error)
	updateTripFn           func(ctx context.Context, arg sqlcdb.UpdateTripParams) error
	deleteTripFn           func(ctx context.Context, id int64) error
	cancelTripFn           func(ctx context.Context, id int64) (sqlcdb.Trip, error)
	getTripStatusFn        func(ctx context.Context, id int64) (sqlcdb.TripStatus, error)
	setTripEnrollTokenFn   func(ctx context.Context, arg sqlcdb.SetTripEnrollTokenParams) error
	clearTripEnrollTokenFn func(ctx context.Context, id int64) error
	getTripByEnrollFn      func(ctx context.Context, token types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error)
	getCruiseByEnrollFn    func(ctx context.Context, token types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error)
	listCruiseTripsFn      func(ctx context.Context, cruiseID types.NullInt64) ([]sqlcdb.Trip, error)
	listCruiseVoyagesFn    func(ctx context.Context, cruiseID types.NullInt64) ([]sqlcdb.Voyage, error)

	// voyages
	listVoyagesFn      func(ctx context.Context, arg sqlcdb.ListVoyagesParams) ([]sqlcdb.Voyage, error)
	countVoyagesFn     func(ctx context.Context) (int64, error)
	getVoyageFn        func(ctx context.Context, id int64) (sqlcdb.Voyage, error)
	createVoyageFn     func(ctx context.Context, arg sqlcdb.CreateVoyageParams) (sqlcdb.Voyage, error)
	updateVoyageFn     func(ctx context.Context, arg sqlcdb.UpdateVoyageParams) error
	deleteVoyageFn     func(ctx context.Context, id int64) error
	getDashboardFn     func(ctx context.Context) (sqlcdb.GetDashboardStatsRow, error)
	getVoyagesByYearFn func(ctx context.Context) ([]sqlcdb.GetVoyagesByYearRow, error)

	// voyage ports
	createVoyagePortFn      func(ctx context.Context, arg sqlcdb.CreateVoyagePortParams) (sqlcdb.VoyagePort, error)
	listVoyagePortsFn       func(ctx context.Context, voyageID int64) ([]sqlcdb.VoyagePort, error)
	deleteVoyagePortFn      func(ctx context.Context, arg sqlcdb.DeleteVoyagePortParams) error
	setVoyagePortPositionFn func(ctx context.Context, arg sqlcdb.SetVoyagePortPositionParams) error

	// crew assignments
	createTripCrewFn         func(ctx context.Context, arg sqlcdb.CreateTripCrewAssignmentParams) (sqlcdb.CrewAssignment, error)
	listTripCrewFn           func(ctx context.Context, tripID types.NullInt64) ([]sqlcdb.ListTripCrewAssignmentsRow, error)
	deleteTripCrewFn         func(ctx context.Context, arg sqlcdb.DeleteTripCrewAssignmentParams) error
	createVoyageCrewFn       func(ctx context.Context, arg sqlcdb.CreateVoyageCrewAssignmentParams) (sqlcdb.CrewAssignment, error)
	listVoyageCrewFn         func(ctx context.Context, voyageID types.NullInt64) ([]sqlcdb.ListVoyageCrewAssignmentsRow, error)
	deleteVoyageCrewFn       func(ctx context.Context, arg sqlcdb.DeleteVoyageCrewAssignmentParams) error
	getVoyageCrewByMemberFn  func(ctx context.Context, arg sqlcdb.GetVoyageCrewAssignmentByMemberParams) (sqlcdb.GetVoyageCrewAssignmentByMemberRow, error)
	getCrewMemberStatsFn     func(ctx context.Context, crewMemberID int64) (sqlcdb.GetCrewMemberStatsRow, error)
	getCrewMemberTripsFn     func(ctx context.Context, crewMemberID int64) ([]sqlcdb.GetCrewMemberTripsRow, error)
	getCrewMemberVoyagesFn   func(ctx context.Context, crewMemberID int64) ([]sqlcdb.GetCrewMemberVoyagesRow, error)
	repointCrewAssignmentsFn func(ctx context.Context, arg sqlcdb.RepointCrewAssignmentsToVoyageParams) error

	// trip enrollments
	createTripEnrollmentFn         func(ctx context.Context, arg sqlcdb.CreateTripEnrollmentParams) (sqlcdb.TripEnrollment, error)
	listTripEnrollmentsFn          func(ctx context.Context, tripID int64) ([]sqlcdb.ListTripEnrollmentsRow, error)
	updateTripEnrollmentStatusFn   func(ctx context.Context, arg sqlcdb.UpdateTripEnrollmentStatusParams) error
	deleteTripEnrollmentFn         func(ctx context.Context, id int64) error
	countTripEnrollmentsFn         func(ctx context.Context, tripID int64) (sqlcdb.CountTripEnrollmentsRow, error)
	getUserTripEnrollmentFn        func(ctx context.Context, arg sqlcdb.GetUserTripEnrollmentParams) (sqlcdb.TripEnrollment, error)
	deleteTripEnrollmentsForTripFn func(ctx context.Context, tripID int64) error

	// cruise enrollments
	createCruiseEnrollmentFn       func(ctx context.Context, arg sqlcdb.CreateCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error)
	listCruiseEnrollmentsFn        func(ctx context.Context, cruiseID int64) ([]sqlcdb.ListCruiseEnrollmentsRow, error)
	updateCruiseEnrollmentStatusFn func(ctx context.Context, arg sqlcdb.UpdateCruiseEnrollmentStatusParams) error
	assignCruiseEnrollmentFn       func(ctx context.Context, arg sqlcdb.AssignCruiseEnrollmentToTripParams) error
	deleteCruiseEnrollmentFn       func(ctx context.Context, id int64) error
	countCruiseEnrollmentsFn       func(ctx context.Context, cruiseID int64) (sqlcdb.CountCruiseEnrollmentsRow, error)
	getUserCruiseEnrollmentFn      func(ctx context.Context, arg sqlcdb.GetUserCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error)

	// voyage opinions
	createVoyageOpinionFn      func(ctx context.Context, arg sqlcdb.CreateVoyageOpinionParams) (sqlcdb.VoyageOpinion, error)
	upsertVoyageOpinionFn      func(ctx context.Context, arg sqlcdb.UpsertVoyageOpinionParams) (sqlcdb.VoyageOpinion, error)
	listVoyageVoyageOpinionsFn func(ctx context.Context, voyageID int64) ([]sqlcdb.ListVoyageVoyageOpinionsRow, error)
	getVoyageOpinionFn         func(ctx context.Context, id int64) (sqlcdb.VoyageOpinion, error)
	deleteVoyageOpinionFn      func(ctx context.Context, id int64) error

	// yachts
	listYachtsFn     func(ctx context.Context, arg sqlcdb.ListYachtsParams) ([]sqlcdb.Yacht, error)
	countYachtsFn    func(ctx context.Context) (int64, error)
	getYachtFn       func(ctx context.Context, id int64) (sqlcdb.Yacht, error)
	createYachtFn    func(ctx context.Context, arg sqlcdb.CreateYachtParams) (sqlcdb.Yacht, error)
	updateYachtFn    func(ctx context.Context, arg sqlcdb.UpdateYachtParams) error
	deleteYachtFn    func(ctx context.Context, id int64) error
	getYachtByNameFn func(ctx context.Context, name string) (sqlcdb.Yacht, error)

	// trainings
	listTrainingsFn  func(ctx context.Context, arg sqlcdb.ListTrainingsParams) ([]sqlcdb.Training, error)
	countTrainingsFn func(ctx context.Context, userID int64) (int64, error)
	getTrainingFn    func(ctx context.Context, arg sqlcdb.GetTrainingParams) (sqlcdb.Training, error)
	createTrainingFn func(ctx context.Context, arg sqlcdb.CreateTrainingParams) (sqlcdb.Training, error)
	updateTrainingFn func(ctx context.Context, arg sqlcdb.UpdateTrainingParams) error
	deleteTrainingFn func(ctx context.Context, arg sqlcdb.DeleteTrainingParams) error

	// crew members
	listCrewMembersFn     func(ctx context.Context, arg sqlcdb.ListCrewMembersParams) ([]sqlcdb.CrewMember, error)
	countCrewMembersFn    func(ctx context.Context) (int64, error)
	getCrewMemberFn       func(ctx context.Context, id int64) (sqlcdb.CrewMember, error)
	createCrewMemberFn    func(ctx context.Context, arg sqlcdb.CreateCrewMemberParams) (sqlcdb.CrewMember, error)
	updateCrewMemberFn    func(ctx context.Context, arg sqlcdb.UpdateCrewMemberParams) error
	deleteCrewMemberFn    func(ctx context.Context, id int64) error
	getCrewMemberByNameFn func(ctx context.Context, fullName string) (sqlcdb.CrewMember, error)

	// cruises
	listCruisesFn          func(ctx context.Context, arg sqlcdb.ListCruisesParams) ([]sqlcdb.Cruise, error)
	countCruisesFn         func(ctx context.Context) (int64, error)
	getCruiseFn            func(ctx context.Context, id int64) (sqlcdb.Cruise, error)
	createCruiseFn         func(ctx context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error)
	updateCruiseFn         func(ctx context.Context, arg sqlcdb.UpdateCruiseParams) error
	deleteCruiseFn         func(ctx context.Context, id int64) error
	setCruiseEnrollToken   func(ctx context.Context, arg sqlcdb.SetCruiseEnrollTokenParams) error
	clearCruiseEnrollToken func(ctx context.Context, id int64) error

	// users
	getUserByIDFn      func(ctx context.Context, id int64) (sqlcdb.User, error)
	updateUserFn       func(ctx context.Context, arg sqlcdb.UpdateUserParams) error
	updateUserPatentFn func(ctx context.Context, arg sqlcdb.UpdateUserPatentParams) error
	updateUserRoleFn   func(ctx context.Context, arg sqlcdb.UpdateUserRoleParams) error
	listUsersFn        func(ctx context.Context) ([]sqlcdb.User, error)
	countUsersFn       func(ctx context.Context) (int64, error)
	countAdminsFn      func(ctx context.Context) (int64, error)
}

// --- trips ---

func (m *mockQuerier) ListTrips(ctx context.Context, arg sqlcdb.ListTripsParams) ([]sqlcdb.Trip, error) {
	if m.listTripsFn == nil {
		panic("unexpected call to ListTrips")
	}
	return m.listTripsFn(ctx, arg)
}

func (m *mockQuerier) CountTrips(ctx context.Context) (int64, error) {
	if m.countTripsFn == nil {
		panic("unexpected call to CountTrips")
	}
	return m.countTripsFn(ctx)
}

func (m *mockQuerier) GetTrip(ctx context.Context, id int64) (sqlcdb.Trip, error) {
	if m.getTripFn == nil {
		panic("unexpected call to GetTrip")
	}
	return m.getTripFn(ctx, id)
}

func (m *mockQuerier) CreateTrip(ctx context.Context, arg sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
	if m.createTripFn == nil {
		panic("unexpected call to CreateTrip")
	}
	return m.createTripFn(ctx, arg)
}

func (m *mockQuerier) UpdateTrip(ctx context.Context, arg sqlcdb.UpdateTripParams) error {
	if m.updateTripFn == nil {
		panic("unexpected call to UpdateTrip")
	}
	return m.updateTripFn(ctx, arg)
}

func (m *mockQuerier) DeleteTrip(ctx context.Context, id int64) error {
	if m.deleteTripFn == nil {
		panic("unexpected call to DeleteTrip")
	}
	return m.deleteTripFn(ctx, id)
}

func (m *mockQuerier) CancelTrip(ctx context.Context, id int64) (sqlcdb.Trip, error) {
	if m.cancelTripFn == nil {
		panic("unexpected call to CancelTrip")
	}
	return m.cancelTripFn(ctx, id)
}

func (m *mockQuerier) GetTripStatus(ctx context.Context, id int64) (sqlcdb.TripStatus, error) {
	if m.getTripStatusFn == nil {
		panic("unexpected call to GetTripStatus")
	}
	return m.getTripStatusFn(ctx, id)
}

func (m *mockQuerier) SetTripEnrollToken(ctx context.Context, arg sqlcdb.SetTripEnrollTokenParams) error {
	if m.setTripEnrollTokenFn == nil {
		panic("unexpected call to SetTripEnrollToken")
	}
	return m.setTripEnrollTokenFn(ctx, arg)
}

func (m *mockQuerier) ClearTripEnrollToken(ctx context.Context, id int64) error {
	if m.clearTripEnrollTokenFn == nil {
		panic("unexpected call to ClearTripEnrollToken")
	}
	return m.clearTripEnrollTokenFn(ctx, id)
}

func (m *mockQuerier) GetTripByEnrollToken(ctx context.Context, token types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
	if m.getTripByEnrollFn == nil {
		panic("unexpected call to GetTripByEnrollToken")
	}
	return m.getTripByEnrollFn(ctx, token)
}

func (m *mockQuerier) ListCruiseTrips(ctx context.Context, cruiseID types.NullInt64) ([]sqlcdb.Trip, error) {
	if m.listCruiseTripsFn == nil {
		panic("unexpected call to ListCruiseTrips")
	}
	return m.listCruiseTripsFn(ctx, cruiseID)
}

func (m *mockQuerier) ListCruiseVoyages(ctx context.Context, cruiseID types.NullInt64) ([]sqlcdb.Voyage, error) {
	if m.listCruiseVoyagesFn == nil {
		panic("unexpected call to ListCruiseVoyages")
	}
	return m.listCruiseVoyagesFn(ctx, cruiseID)
}

// --- voyages ---

func (m *mockQuerier) ListVoyages(ctx context.Context, arg sqlcdb.ListVoyagesParams) ([]sqlcdb.Voyage, error) {
	if m.listVoyagesFn == nil {
		panic("unexpected call to ListVoyages")
	}
	return m.listVoyagesFn(ctx, arg)
}

func (m *mockQuerier) CountVoyages(ctx context.Context) (int64, error) {
	if m.countVoyagesFn == nil {
		panic("unexpected call to CountVoyages")
	}
	return m.countVoyagesFn(ctx)
}

func (m *mockQuerier) GetVoyage(ctx context.Context, id int64) (sqlcdb.Voyage, error) {
	if m.getVoyageFn == nil {
		panic("unexpected call to GetVoyage")
	}
	return m.getVoyageFn(ctx, id)
}

func (m *mockQuerier) CreateVoyage(ctx context.Context, arg sqlcdb.CreateVoyageParams) (sqlcdb.Voyage, error) {
	if m.createVoyageFn == nil {
		panic("unexpected call to CreateVoyage")
	}
	return m.createVoyageFn(ctx, arg)
}

func (m *mockQuerier) UpdateVoyage(ctx context.Context, arg sqlcdb.UpdateVoyageParams) error {
	if m.updateVoyageFn == nil {
		panic("unexpected call to UpdateVoyage")
	}
	return m.updateVoyageFn(ctx, arg)
}

func (m *mockQuerier) DeleteVoyage(ctx context.Context, id int64) error {
	if m.deleteVoyageFn == nil {
		panic("unexpected call to DeleteVoyage")
	}
	return m.deleteVoyageFn(ctx, id)
}

func (m *mockQuerier) GetDashboardStats(ctx context.Context) (sqlcdb.GetDashboardStatsRow, error) {
	if m.getDashboardFn == nil {
		panic("unexpected call to GetDashboardStats")
	}
	return m.getDashboardFn(ctx)
}

func (m *mockQuerier) GetVoyagesByYear(ctx context.Context) ([]sqlcdb.GetVoyagesByYearRow, error) {
	if m.getVoyagesByYearFn == nil {
		panic("unexpected call to GetVoyagesByYear")
	}
	return m.getVoyagesByYearFn(ctx)
}

// --- voyage ports ---

func (m *mockQuerier) CreateVoyagePort(ctx context.Context, arg sqlcdb.CreateVoyagePortParams) (sqlcdb.VoyagePort, error) {
	if m.createVoyagePortFn == nil {
		panic("unexpected call to CreateVoyagePort")
	}
	return m.createVoyagePortFn(ctx, arg)
}

func (m *mockQuerier) ListVoyagePorts(ctx context.Context, voyageID int64) ([]sqlcdb.VoyagePort, error) {
	if m.listVoyagePortsFn == nil {
		panic("unexpected call to ListVoyagePorts")
	}
	return m.listVoyagePortsFn(ctx, voyageID)
}

func (m *mockQuerier) DeleteVoyagePort(ctx context.Context, arg sqlcdb.DeleteVoyagePortParams) error {
	if m.deleteVoyagePortFn == nil {
		panic("unexpected call to DeleteVoyagePort")
	}
	return m.deleteVoyagePortFn(ctx, arg)
}

func (m *mockQuerier) SetVoyagePortPosition(ctx context.Context, arg sqlcdb.SetVoyagePortPositionParams) error {
	if m.setVoyagePortPositionFn == nil {
		panic("unexpected call to SetVoyagePortPosition")
	}
	return m.setVoyagePortPositionFn(ctx, arg)
}

// --- crew assignments ---

func (m *mockQuerier) CreateTripCrewAssignment(ctx context.Context, arg sqlcdb.CreateTripCrewAssignmentParams) (sqlcdb.CrewAssignment, error) {
	if m.createTripCrewFn == nil {
		panic("unexpected call to CreateTripCrewAssignment")
	}
	return m.createTripCrewFn(ctx, arg)
}

func (m *mockQuerier) ListTripCrewAssignments(ctx context.Context, tripID types.NullInt64) ([]sqlcdb.ListTripCrewAssignmentsRow, error) {
	if m.listTripCrewFn == nil {
		panic("unexpected call to ListTripCrewAssignments")
	}
	return m.listTripCrewFn(ctx, tripID)
}

func (m *mockQuerier) DeleteTripCrewAssignment(ctx context.Context, arg sqlcdb.DeleteTripCrewAssignmentParams) error {
	if m.deleteTripCrewFn == nil {
		panic("unexpected call to DeleteTripCrewAssignment")
	}
	return m.deleteTripCrewFn(ctx, arg)
}

func (m *mockQuerier) CreateVoyageCrewAssignment(ctx context.Context, arg sqlcdb.CreateVoyageCrewAssignmentParams) (sqlcdb.CrewAssignment, error) {
	if m.createVoyageCrewFn == nil {
		panic("unexpected call to CreateVoyageCrewAssignment")
	}
	return m.createVoyageCrewFn(ctx, arg)
}

func (m *mockQuerier) ListVoyageCrewAssignments(ctx context.Context, voyageID types.NullInt64) ([]sqlcdb.ListVoyageCrewAssignmentsRow, error) {
	if m.listVoyageCrewFn == nil {
		panic("unexpected call to ListVoyageCrewAssignments")
	}
	return m.listVoyageCrewFn(ctx, voyageID)
}

func (m *mockQuerier) DeleteVoyageCrewAssignment(ctx context.Context, arg sqlcdb.DeleteVoyageCrewAssignmentParams) error {
	if m.deleteVoyageCrewFn == nil {
		panic("unexpected call to DeleteVoyageCrewAssignment")
	}
	return m.deleteVoyageCrewFn(ctx, arg)
}

func (m *mockQuerier) GetVoyageCrewAssignmentByMember(ctx context.Context, arg sqlcdb.GetVoyageCrewAssignmentByMemberParams) (sqlcdb.GetVoyageCrewAssignmentByMemberRow, error) {
	if m.getVoyageCrewByMemberFn == nil {
		panic("unexpected call to GetVoyageCrewAssignmentByMember")
	}
	return m.getVoyageCrewByMemberFn(ctx, arg)
}

func (m *mockQuerier) GetCrewMemberStats(ctx context.Context, crewMemberID int64) (sqlcdb.GetCrewMemberStatsRow, error) {
	if m.getCrewMemberStatsFn == nil {
		panic("unexpected call to GetCrewMemberStats")
	}
	return m.getCrewMemberStatsFn(ctx, crewMemberID)
}

func (m *mockQuerier) GetCrewMemberTrips(ctx context.Context, crewMemberID int64) ([]sqlcdb.GetCrewMemberTripsRow, error) {
	if m.getCrewMemberTripsFn == nil {
		panic("unexpected call to GetCrewMemberTrips")
	}
	return m.getCrewMemberTripsFn(ctx, crewMemberID)
}

func (m *mockQuerier) GetCrewMemberVoyages(ctx context.Context, crewMemberID int64) ([]sqlcdb.GetCrewMemberVoyagesRow, error) {
	if m.getCrewMemberVoyagesFn == nil {
		panic("unexpected call to GetCrewMemberVoyages")
	}
	return m.getCrewMemberVoyagesFn(ctx, crewMemberID)
}

func (m *mockQuerier) RepointCrewAssignmentsToVoyage(ctx context.Context, arg sqlcdb.RepointCrewAssignmentsToVoyageParams) error {
	if m.repointCrewAssignmentsFn == nil {
		panic("unexpected call to RepointCrewAssignmentsToVoyage")
	}
	return m.repointCrewAssignmentsFn(ctx, arg)
}

// --- trip enrollments ---

func (m *mockQuerier) CreateTripEnrollment(ctx context.Context, arg sqlcdb.CreateTripEnrollmentParams) (sqlcdb.TripEnrollment, error) {
	if m.createTripEnrollmentFn == nil {
		panic("unexpected call to CreateTripEnrollment")
	}
	return m.createTripEnrollmentFn(ctx, arg)
}

func (m *mockQuerier) ListTripEnrollments(ctx context.Context, tripID int64) ([]sqlcdb.ListTripEnrollmentsRow, error) {
	if m.listTripEnrollmentsFn == nil {
		panic("unexpected call to ListTripEnrollments")
	}
	return m.listTripEnrollmentsFn(ctx, tripID)
}

func (m *mockQuerier) UpdateTripEnrollmentStatus(ctx context.Context, arg sqlcdb.UpdateTripEnrollmentStatusParams) error {
	if m.updateTripEnrollmentStatusFn == nil {
		panic("unexpected call to UpdateTripEnrollmentStatus")
	}
	return m.updateTripEnrollmentStatusFn(ctx, arg)
}

func (m *mockQuerier) DeleteTripEnrollment(ctx context.Context, id int64) error {
	if m.deleteTripEnrollmentFn == nil {
		panic("unexpected call to DeleteTripEnrollment")
	}
	return m.deleteTripEnrollmentFn(ctx, id)
}

func (m *mockQuerier) CountTripEnrollments(ctx context.Context, tripID int64) (sqlcdb.CountTripEnrollmentsRow, error) {
	if m.countTripEnrollmentsFn == nil {
		panic("unexpected call to CountTripEnrollments")
	}
	return m.countTripEnrollmentsFn(ctx, tripID)
}

func (m *mockQuerier) GetUserTripEnrollment(ctx context.Context, arg sqlcdb.GetUserTripEnrollmentParams) (sqlcdb.TripEnrollment, error) {
	if m.getUserTripEnrollmentFn == nil {
		panic("unexpected call to GetUserTripEnrollment")
	}
	return m.getUserTripEnrollmentFn(ctx, arg)
}

func (m *mockQuerier) DeleteTripEnrollmentsForTrip(ctx context.Context, tripID int64) error {
	if m.deleteTripEnrollmentsForTripFn == nil {
		panic("unexpected call to DeleteTripEnrollmentsForTrip")
	}
	return m.deleteTripEnrollmentsForTripFn(ctx, tripID)
}

// --- cruise enrollments ---

func (m *mockQuerier) CreateCruiseEnrollment(ctx context.Context, arg sqlcdb.CreateCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
	if m.createCruiseEnrollmentFn == nil {
		panic("unexpected call to CreateCruiseEnrollment")
	}
	return m.createCruiseEnrollmentFn(ctx, arg)
}

func (m *mockQuerier) ListCruiseEnrollments(ctx context.Context, cruiseID int64) ([]sqlcdb.ListCruiseEnrollmentsRow, error) {
	if m.listCruiseEnrollmentsFn == nil {
		panic("unexpected call to ListCruiseEnrollments")
	}
	return m.listCruiseEnrollmentsFn(ctx, cruiseID)
}

func (m *mockQuerier) UpdateCruiseEnrollmentStatus(ctx context.Context, arg sqlcdb.UpdateCruiseEnrollmentStatusParams) error {
	if m.updateCruiseEnrollmentStatusFn == nil {
		panic("unexpected call to UpdateCruiseEnrollmentStatus")
	}
	return m.updateCruiseEnrollmentStatusFn(ctx, arg)
}

func (m *mockQuerier) AssignCruiseEnrollmentToTrip(ctx context.Context, arg sqlcdb.AssignCruiseEnrollmentToTripParams) error {
	if m.assignCruiseEnrollmentFn == nil {
		panic("unexpected call to AssignCruiseEnrollmentToTrip")
	}
	return m.assignCruiseEnrollmentFn(ctx, arg)
}

func (m *mockQuerier) DeleteCruiseEnrollment(ctx context.Context, id int64) error {
	if m.deleteCruiseEnrollmentFn == nil {
		panic("unexpected call to DeleteCruiseEnrollment")
	}
	return m.deleteCruiseEnrollmentFn(ctx, id)
}

func (m *mockQuerier) CountCruiseEnrollments(ctx context.Context, cruiseID int64) (sqlcdb.CountCruiseEnrollmentsRow, error) {
	if m.countCruiseEnrollmentsFn == nil {
		panic("unexpected call to CountCruiseEnrollments")
	}
	return m.countCruiseEnrollmentsFn(ctx, cruiseID)
}

func (m *mockQuerier) GetUserCruiseEnrollment(ctx context.Context, arg sqlcdb.GetUserCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
	if m.getUserCruiseEnrollmentFn == nil {
		panic("unexpected call to GetUserCruiseEnrollment")
	}
	return m.getUserCruiseEnrollmentFn(ctx, arg)
}

// --- voyage opinions ---

func (m *mockQuerier) CreateVoyageOpinion(ctx context.Context, arg sqlcdb.CreateVoyageOpinionParams) (sqlcdb.VoyageOpinion, error) {
	if m.createVoyageOpinionFn == nil {
		panic("unexpected call to CreateVoyageOpinion")
	}
	return m.createVoyageOpinionFn(ctx, arg)
}

func (m *mockQuerier) UpsertVoyageOpinion(ctx context.Context, arg sqlcdb.UpsertVoyageOpinionParams) (sqlcdb.VoyageOpinion, error) {
	if m.upsertVoyageOpinionFn == nil {
		panic("unexpected call to UpsertVoyageOpinion")
	}
	return m.upsertVoyageOpinionFn(ctx, arg)
}

func (m *mockQuerier) ListVoyageVoyageOpinions(ctx context.Context, voyageID int64) ([]sqlcdb.ListVoyageVoyageOpinionsRow, error) {
	if m.listVoyageVoyageOpinionsFn == nil {
		panic("unexpected call to ListVoyageVoyageOpinions")
	}
	return m.listVoyageVoyageOpinionsFn(ctx, voyageID)
}

func (m *mockQuerier) GetVoyageOpinion(ctx context.Context, id int64) (sqlcdb.VoyageOpinion, error) {
	if m.getVoyageOpinionFn == nil {
		panic("unexpected call to GetVoyageOpinion")
	}
	return m.getVoyageOpinionFn(ctx, id)
}

func (m *mockQuerier) DeleteVoyageOpinion(ctx context.Context, id int64) error {
	if m.deleteVoyageOpinionFn == nil {
		panic("unexpected call to DeleteVoyageOpinion")
	}
	return m.deleteVoyageOpinionFn(ctx, id)
}

// --- yachts ---

func (m *mockQuerier) ListYachts(ctx context.Context, arg sqlcdb.ListYachtsParams) ([]sqlcdb.Yacht, error) {
	if m.listYachtsFn == nil {
		panic("unexpected call to ListYachts")
	}
	return m.listYachtsFn(ctx, arg)
}

func (m *mockQuerier) CountYachts(ctx context.Context) (int64, error) {
	if m.countYachtsFn == nil {
		panic("unexpected call to CountYachts")
	}
	return m.countYachtsFn(ctx)
}

func (m *mockQuerier) GetYacht(ctx context.Context, id int64) (sqlcdb.Yacht, error) {
	if m.getYachtFn == nil {
		panic("unexpected call to GetYacht")
	}
	return m.getYachtFn(ctx, id)
}

func (m *mockQuerier) CreateYacht(ctx context.Context, arg sqlcdb.CreateYachtParams) (sqlcdb.Yacht, error) {
	if m.createYachtFn == nil {
		panic("unexpected call to CreateYacht")
	}
	return m.createYachtFn(ctx, arg)
}

func (m *mockQuerier) UpdateYacht(ctx context.Context, arg sqlcdb.UpdateYachtParams) error {
	if m.updateYachtFn == nil {
		panic("unexpected call to UpdateYacht")
	}
	return m.updateYachtFn(ctx, arg)
}

func (m *mockQuerier) DeleteYacht(ctx context.Context, id int64) error {
	if m.deleteYachtFn == nil {
		panic("unexpected call to DeleteYacht")
	}
	return m.deleteYachtFn(ctx, id)
}

func (m *mockQuerier) GetYachtByName(ctx context.Context, name string) (sqlcdb.Yacht, error) {
	if m.getYachtByNameFn == nil {
		panic("unexpected call to GetYachtByName")
	}
	return m.getYachtByNameFn(ctx, name)
}

// --- trainings ---

func (m *mockQuerier) ListTrainings(ctx context.Context, arg sqlcdb.ListTrainingsParams) ([]sqlcdb.Training, error) {
	if m.listTrainingsFn == nil {
		panic("unexpected call to ListTrainings")
	}
	return m.listTrainingsFn(ctx, arg)
}

func (m *mockQuerier) CountTrainings(ctx context.Context, userID int64) (int64, error) {
	if m.countTrainingsFn == nil {
		panic("unexpected call to CountTrainings")
	}
	return m.countTrainingsFn(ctx, userID)
}

func (m *mockQuerier) GetTraining(ctx context.Context, arg sqlcdb.GetTrainingParams) (sqlcdb.Training, error) {
	if m.getTrainingFn == nil {
		panic("unexpected call to GetTraining")
	}
	return m.getTrainingFn(ctx, arg)
}

func (m *mockQuerier) CreateTraining(ctx context.Context, arg sqlcdb.CreateTrainingParams) (sqlcdb.Training, error) {
	if m.createTrainingFn == nil {
		panic("unexpected call to CreateTraining")
	}
	return m.createTrainingFn(ctx, arg)
}

func (m *mockQuerier) UpdateTraining(ctx context.Context, arg sqlcdb.UpdateTrainingParams) error {
	if m.updateTrainingFn == nil {
		panic("unexpected call to UpdateTraining")
	}
	return m.updateTrainingFn(ctx, arg)
}

func (m *mockQuerier) DeleteTraining(ctx context.Context, arg sqlcdb.DeleteTrainingParams) error {
	if m.deleteTrainingFn == nil {
		panic("unexpected call to DeleteTraining")
	}
	return m.deleteTrainingFn(ctx, arg)
}

// --- crew members ---

func (m *mockQuerier) ListCrewMembers(ctx context.Context, arg sqlcdb.ListCrewMembersParams) ([]sqlcdb.CrewMember, error) {
	if m.listCrewMembersFn == nil {
		panic("unexpected call to ListCrewMembers")
	}
	return m.listCrewMembersFn(ctx, arg)
}

func (m *mockQuerier) CountCrewMembers(ctx context.Context) (int64, error) {
	if m.countCrewMembersFn == nil {
		panic("unexpected call to CountCrewMembers")
	}
	return m.countCrewMembersFn(ctx)
}

func (m *mockQuerier) GetCrewMember(ctx context.Context, id int64) (sqlcdb.CrewMember, error) {
	if m.getCrewMemberFn == nil {
		panic("unexpected call to GetCrewMember")
	}
	return m.getCrewMemberFn(ctx, id)
}

func (m *mockQuerier) CreateCrewMember(ctx context.Context, arg sqlcdb.CreateCrewMemberParams) (sqlcdb.CrewMember, error) {
	if m.createCrewMemberFn == nil {
		panic("unexpected call to CreateCrewMember")
	}
	return m.createCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) UpdateCrewMember(ctx context.Context, arg sqlcdb.UpdateCrewMemberParams) error {
	if m.updateCrewMemberFn == nil {
		panic("unexpected call to UpdateCrewMember")
	}
	return m.updateCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) DeleteCrewMember(ctx context.Context, id int64) error {
	if m.deleteCrewMemberFn == nil {
		panic("unexpected call to DeleteCrewMember")
	}
	return m.deleteCrewMemberFn(ctx, id)
}

func (m *mockQuerier) GetCrewMemberByName(ctx context.Context, fullName string) (sqlcdb.CrewMember, error) {
	if m.getCrewMemberByNameFn == nil {
		panic("unexpected call to GetCrewMemberByName")
	}
	return m.getCrewMemberByNameFn(ctx, fullName)
}

// --- cruises ---

func (m *mockQuerier) ListCruises(ctx context.Context, arg sqlcdb.ListCruisesParams) ([]sqlcdb.Cruise, error) {
	if m.listCruisesFn == nil {
		panic("unexpected call to ListCruises")
	}
	return m.listCruisesFn(ctx, arg)
}

func (m *mockQuerier) CountCruises(ctx context.Context) (int64, error) {
	if m.countCruisesFn == nil {
		panic("unexpected call to CountCruises")
	}
	return m.countCruisesFn(ctx)
}

func (m *mockQuerier) GetCruise(ctx context.Context, id int64) (sqlcdb.Cruise, error) {
	if m.getCruiseFn == nil {
		panic("unexpected call to GetCruise")
	}
	return m.getCruiseFn(ctx, id)
}

func (m *mockQuerier) CreateCruise(ctx context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
	if m.createCruiseFn == nil {
		panic("unexpected call to CreateCruise")
	}
	return m.createCruiseFn(ctx, arg)
}

func (m *mockQuerier) UpdateCruise(ctx context.Context, arg sqlcdb.UpdateCruiseParams) error {
	if m.updateCruiseFn == nil {
		panic("unexpected call to UpdateCruise")
	}
	return m.updateCruiseFn(ctx, arg)
}

func (m *mockQuerier) DeleteCruise(ctx context.Context, id int64) error {
	if m.deleteCruiseFn == nil {
		panic("unexpected call to DeleteCruise")
	}
	return m.deleteCruiseFn(ctx, id)
}

func (m *mockQuerier) SetCruiseEnrollToken(ctx context.Context, arg sqlcdb.SetCruiseEnrollTokenParams) error {
	if m.setCruiseEnrollToken == nil {
		panic("unexpected call to SetCruiseEnrollToken")
	}
	return m.setCruiseEnrollToken(ctx, arg)
}

func (m *mockQuerier) ClearCruiseEnrollToken(ctx context.Context, id int64) error {
	if m.clearCruiseEnrollToken == nil {
		panic("unexpected call to ClearCruiseEnrollToken")
	}
	return m.clearCruiseEnrollToken(ctx, id)
}

func (m *mockQuerier) GetCruiseByEnrollToken(ctx context.Context, token types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
	if m.getCruiseByEnrollFn == nil {
		panic("unexpected call to GetCruiseByEnrollToken")
	}
	return m.getCruiseByEnrollFn(ctx, token)
}

// --- users ---

func (m *mockQuerier) GetUserByID(ctx context.Context, id int64) (sqlcdb.User, error) {
	if m.getUserByIDFn == nil {
		panic("unexpected call to GetUserByID")
	}
	return m.getUserByIDFn(ctx, id)
}

func (m *mockQuerier) UpdateUser(ctx context.Context, arg sqlcdb.UpdateUserParams) error {
	if m.updateUserFn == nil {
		panic("unexpected call to UpdateUser")
	}
	return m.updateUserFn(ctx, arg)
}

func (m *mockQuerier) UpdateUserPatent(ctx context.Context, arg sqlcdb.UpdateUserPatentParams) error {
	if m.updateUserPatentFn == nil {
		panic("unexpected call to UpdateUserPatent")
	}
	return m.updateUserPatentFn(ctx, arg)
}

func (m *mockQuerier) UpdateUserRole(ctx context.Context, arg sqlcdb.UpdateUserRoleParams) error {
	if m.updateUserRoleFn == nil {
		panic("unexpected call to UpdateUserRole")
	}
	return m.updateUserRoleFn(ctx, arg)
}

func (m *mockQuerier) ListUsers(ctx context.Context) ([]sqlcdb.User, error) {
	if m.listUsersFn == nil {
		panic("unexpected call to ListUsers")
	}
	return m.listUsersFn(ctx)
}

func (m *mockQuerier) CountUsers(ctx context.Context) (int64, error) {
	if m.countUsersFn == nil {
		panic("unexpected call to CountUsers")
	}
	return m.countUsersFn(ctx)
}

func (m *mockQuerier) CountAdmins(ctx context.Context) (int64, error) {
	if m.countAdminsFn == nil {
		panic("unexpected call to CountAdmins")
	}
	return m.countAdminsFn(ctx)
}

// --- panics for things never set up in tests ---

func (m *mockQuerier) CreateUser(context.Context, sqlcdb.CreateUserParams) (sqlcdb.User, error) {
	panic("unexpected call to CreateUser")
}

func (m *mockQuerier) GetUserByEmail(context.Context, string) (sqlcdb.User, error) {
	panic("unexpected call to GetUserByEmail")
}

func (m *mockQuerier) GetUserByFirebaseUID(context.Context, types.NullString) (sqlcdb.User, error) {
	panic("unexpected call to GetUserByFirebaseUID")
}

func (m *mockQuerier) LinkFirebaseUIDByEmail(context.Context, sqlcdb.LinkFirebaseUIDByEmailParams) (sqlcdb.User, error) {
	panic("unexpected call to LinkFirebaseUIDByEmail")
}

func (m *mockQuerier) UpsertUserByFirebaseUID(context.Context, sqlcdb.UpsertUserByFirebaseUIDParams) (sqlcdb.User, error) {
	panic("unexpected call to UpsertUserByFirebaseUID")
}

// directTxRunner returns a txRunner that runs fn directly against the given
// mock querier without a real transaction, for testing import write paths.
func directTxRunner(q sqlcdb.Querier) txRunner {
	return func(ctx context.Context, fn func(sqlcdb.Querier) error) error {
		return fn(q)
	}
}

// userCtx injects an admin caller, the default for mutation-path tests.
func userCtx(ctx context.Context) context.Context {
	return userCtxRole(ctx, "admin")
}

// userCtxRole injects an authenticated caller with the given role.
func userCtxRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, middleware.UserCtxKey, &auth.Claims{
		UserID: 1, Email: "test@example.com", Name: "Test User", Role: role,
	})
}
