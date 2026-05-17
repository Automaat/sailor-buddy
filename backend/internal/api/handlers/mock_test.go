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
	listTripsFn            func(ctx context.Context, ownerID int64) ([]sqlcdb.Trip, error)
	getTripFn              func(ctx context.Context, arg sqlcdb.GetTripParams) (sqlcdb.Trip, error)
	createTripFn           func(ctx context.Context, arg sqlcdb.CreateTripParams) (sqlcdb.Trip, error)
	updateTripFn           func(ctx context.Context, arg sqlcdb.UpdateTripParams) error
	deleteTripFn           func(ctx context.Context, arg sqlcdb.DeleteTripParams) error
	cancelTripFn           func(ctx context.Context, arg sqlcdb.CancelTripParams) (sqlcdb.Trip, error)
	getTripStatusFn        func(ctx context.Context, id int64) (sqlcdb.TripStatus, error)
	setTripEnrollTokenFn   func(ctx context.Context, arg sqlcdb.SetTripEnrollTokenParams) error
	clearTripEnrollTokenFn func(ctx context.Context, arg sqlcdb.ClearTripEnrollTokenParams) error
	getTripByEnrollFn      func(ctx context.Context, token types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error)
	getCruiseByEnrollFn    func(ctx context.Context, token types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error)

	// voyages
	listVoyagesFn      func(ctx context.Context, ownerID int64) ([]sqlcdb.Voyage, error)
	getVoyageFn        func(ctx context.Context, arg sqlcdb.GetVoyageParams) (sqlcdb.Voyage, error)
	createVoyageFn     func(ctx context.Context, arg sqlcdb.CreateVoyageParams) (sqlcdb.Voyage, error)
	updateVoyageFn     func(ctx context.Context, arg sqlcdb.UpdateVoyageParams) error
	deleteVoyageFn     func(ctx context.Context, arg sqlcdb.DeleteVoyageParams) error
	getDashboardFn     func(ctx context.Context, ownerID int64) (sqlcdb.GetDashboardStatsRow, error)
	getVoyagesByYearFn func(ctx context.Context, ownerID int64) ([]sqlcdb.GetVoyagesByYearRow, error)

	// voyage ports
	createVoyagePortFn    func(ctx context.Context, arg sqlcdb.CreateVoyagePortParams) (sqlcdb.VoyagePort, error)
	listVoyagePortsFn     func(ctx context.Context, arg sqlcdb.ListVoyagePortsParams) ([]sqlcdb.VoyagePort, error)
	deleteVoyagePortFn    func(ctx context.Context, arg sqlcdb.DeleteVoyagePortParams) error
	listOrgVoyagePortsFn  func(ctx context.Context, arg sqlcdb.ListOrgVoyagePortsParams) ([]sqlcdb.VoyagePort, error)
	deleteOrgVoyagePortFn func(ctx context.Context, arg sqlcdb.DeleteOrgVoyagePortParams) error

	// org trips/voyages
	listOrgTripsFn    func(ctx context.Context, orgID types.NullInt64) ([]sqlcdb.Trip, error)
	getOrgTripFn      func(ctx context.Context, arg sqlcdb.GetOrgTripParams) (sqlcdb.Trip, error)
	createOrgTripFn   func(ctx context.Context, arg sqlcdb.CreateOrgTripParams) (sqlcdb.Trip, error)
	updateOrgTripFn   func(ctx context.Context, arg sqlcdb.UpdateOrgTripParams) error
	deleteOrgTripFn   func(ctx context.Context, arg sqlcdb.DeleteOrgTripParams) error
	cancelOrgTripFn   func(ctx context.Context, arg sqlcdb.CancelOrgTripParams) (sqlcdb.Trip, error)
	listOrgVoyagesFn  func(ctx context.Context, orgID types.NullInt64) ([]sqlcdb.Voyage, error)
	getOrgVoyageFn    func(ctx context.Context, arg sqlcdb.GetOrgVoyageParams) (sqlcdb.Voyage, error)
	createOrgVoyageFn func(ctx context.Context, arg sqlcdb.CreateOrgVoyageParams) (sqlcdb.Voyage, error)
	updateOrgVoyageFn func(ctx context.Context, arg sqlcdb.UpdateOrgVoyageParams) error
	deleteOrgVoyageFn func(ctx context.Context, arg sqlcdb.DeleteOrgVoyageParams) error

	// crew assignments
	createTripCrewFn         func(ctx context.Context, arg sqlcdb.CreateTripCrewAssignmentParams) (sqlcdb.CrewAssignment, error)
	listTripCrewFn           func(ctx context.Context, arg sqlcdb.ListTripCrewAssignmentsParams) ([]sqlcdb.ListTripCrewAssignmentsRow, error)
	deleteTripCrewFn         func(ctx context.Context, arg sqlcdb.DeleteTripCrewAssignmentParams) error
	createVoyageCrewFn       func(ctx context.Context, arg sqlcdb.CreateVoyageCrewAssignmentParams) (sqlcdb.CrewAssignment, error)
	listVoyageCrewFn         func(ctx context.Context, arg sqlcdb.ListVoyageCrewAssignmentsParams) ([]sqlcdb.ListVoyageCrewAssignmentsRow, error)
	deleteVoyageCrewFn       func(ctx context.Context, arg sqlcdb.DeleteVoyageCrewAssignmentParams) error
	listOrgTripCrewFn        func(ctx context.Context, arg sqlcdb.ListOrgTripCrewAssignmentsParams) ([]sqlcdb.ListOrgTripCrewAssignmentsRow, error)
	deleteOrgTripCrewFn      func(ctx context.Context, arg sqlcdb.DeleteOrgTripCrewAssignmentParams) error
	listOrgVoyageCrewFn      func(ctx context.Context, arg sqlcdb.ListOrgVoyageCrewAssignmentsParams) ([]sqlcdb.ListOrgVoyageCrewAssignmentsRow, error)
	deleteOrgVoyageCrewFn    func(ctx context.Context, arg sqlcdb.DeleteOrgVoyageCrewAssignmentParams) error
	getVoyageCrewByMemberFn  func(ctx context.Context, arg sqlcdb.GetVoyageCrewAssignmentByMemberParams) (sqlcdb.GetVoyageCrewAssignmentByMemberRow, error)
	getCrewMemberStatsFn     func(ctx context.Context, crewMemberID int64) (sqlcdb.GetCrewMemberStatsRow, error)
	getCrewMemberTripsFn     func(ctx context.Context, crewMemberID int64) ([]sqlcdb.GetCrewMemberTripsRow, error)
	getCrewMemberVoyagesFn   func(ctx context.Context, crewMemberID int64) ([]sqlcdb.GetCrewMemberVoyagesRow, error)
	repointCrewAssignmentsFn func(ctx context.Context, arg sqlcdb.RepointCrewAssignmentsToVoyageParams) error

	// trip enrollments
	createTripEnrollmentFn         func(ctx context.Context, arg sqlcdb.CreateTripEnrollmentParams) (sqlcdb.TripEnrollment, error)
	listTripEnrollmentsFn          func(ctx context.Context, arg sqlcdb.ListTripEnrollmentsParams) ([]sqlcdb.ListTripEnrollmentsRow, error)
	updateTripEnrollmentStatusFn   func(ctx context.Context, arg sqlcdb.UpdateTripEnrollmentStatusParams) error
	deleteTripEnrollmentFn         func(ctx context.Context, arg sqlcdb.DeleteTripEnrollmentParams) error
	countTripEnrollmentsFn         func(ctx context.Context, tripID int64) (sqlcdb.CountTripEnrollmentsRow, error)
	getUserTripEnrollmentFn        func(ctx context.Context, arg sqlcdb.GetUserTripEnrollmentParams) (sqlcdb.TripEnrollment, error)
	deleteTripEnrollmentsForTripFn func(ctx context.Context, tripID int64) error

	// voyage opinions
	createVoyageOpinionFn      func(ctx context.Context, arg sqlcdb.CreateVoyageOpinionParams) (sqlcdb.VoyageOpinion, error)
	upsertVoyageOpinionFn      func(ctx context.Context, arg sqlcdb.UpsertVoyageOpinionParams) (sqlcdb.VoyageOpinion, error)
	listVoyageVoyageOpinionsFn func(ctx context.Context, voyageID int64) ([]sqlcdb.ListVoyageVoyageOpinionsRow, error)
	getVoyageOpinionFn         func(ctx context.Context, id int64) (sqlcdb.VoyageOpinion, error)
	deleteVoyageOpinionFn      func(ctx context.Context, id int64) error

	// yachts
	listYachtsFn     func(ctx context.Context, ownerID int64) ([]sqlcdb.Yacht, error)
	getYachtFn       func(ctx context.Context, arg sqlcdb.GetYachtParams) (sqlcdb.Yacht, error)
	createYachtFn    func(ctx context.Context, arg sqlcdb.CreateYachtParams) (sqlcdb.Yacht, error)
	updateYachtFn    func(ctx context.Context, arg sqlcdb.UpdateYachtParams) error
	deleteYachtFn    func(ctx context.Context, arg sqlcdb.DeleteYachtParams) error
	getYachtByNameFn func(ctx context.Context, arg sqlcdb.GetYachtByNameParams) (sqlcdb.Yacht, error)
	listOrgYachtsFn  func(ctx context.Context, orgID types.NullInt64) ([]sqlcdb.Yacht, error)
	getOrgYachtFn    func(ctx context.Context, arg sqlcdb.GetOrgYachtParams) (sqlcdb.Yacht, error)
	createOrgYachtFn func(ctx context.Context, arg sqlcdb.CreateOrgYachtParams) (sqlcdb.Yacht, error)
	updateOrgYachtFn func(ctx context.Context, arg sqlcdb.UpdateOrgYachtParams) error
	deleteOrgYachtFn func(ctx context.Context, arg sqlcdb.DeleteOrgYachtParams) error

	// trainings
	listTrainingsFn  func(ctx context.Context, userID int64) ([]sqlcdb.Training, error)
	getTrainingFn    func(ctx context.Context, arg sqlcdb.GetTrainingParams) (sqlcdb.Training, error)
	createTrainingFn func(ctx context.Context, arg sqlcdb.CreateTrainingParams) (sqlcdb.Training, error)
	updateTrainingFn func(ctx context.Context, arg sqlcdb.UpdateTrainingParams) error
	deleteTrainingFn func(ctx context.Context, arg sqlcdb.DeleteTrainingParams) error

	// crew members
	listCrewMembersFn     func(ctx context.Context, ownerID int64) ([]sqlcdb.CrewMember, error)
	getCrewMemberFn       func(ctx context.Context, arg sqlcdb.GetCrewMemberParams) (sqlcdb.CrewMember, error)
	createCrewMemberFn    func(ctx context.Context, arg sqlcdb.CreateCrewMemberParams) (sqlcdb.CrewMember, error)
	updateCrewMemberFn    func(ctx context.Context, arg sqlcdb.UpdateCrewMemberParams) error
	deleteCrewMemberFn    func(ctx context.Context, arg sqlcdb.DeleteCrewMemberParams) error
	getCrewMemberByNameFn func(ctx context.Context, arg sqlcdb.GetCrewMemberByNameParams) (sqlcdb.CrewMember, error)
	listOrgCrewMembersFn  func(ctx context.Context, orgID types.NullInt64) ([]sqlcdb.CrewMember, error)
	getOrgCrewMemberFn    func(ctx context.Context, arg sqlcdb.GetOrgCrewMemberParams) (sqlcdb.CrewMember, error)
	createOrgCrewMemberFn func(ctx context.Context, arg sqlcdb.CreateOrgCrewMemberParams) (sqlcdb.CrewMember, error)
	updateOrgCrewMemberFn func(ctx context.Context, arg sqlcdb.UpdateOrgCrewMemberParams) error
	deleteOrgCrewMemberFn func(ctx context.Context, arg sqlcdb.DeleteOrgCrewMemberParams) error

	// organizations
	listUserOrganizationsFn   func(ctx context.Context, userID int64) ([]sqlcdb.ListUserOrganizationsRow, error)
	createOrganizationFn      func(ctx context.Context, arg sqlcdb.CreateOrganizationParams) (sqlcdb.Organization, error)
	addOrgMemberFn            func(ctx context.Context, arg sqlcdb.AddOrgMemberParams) (sqlcdb.OrgMember, error)
	getOrganizationBySlugFn   func(ctx context.Context, slug string) (sqlcdb.Organization, error)
	updateOrganizationFn      func(ctx context.Context, arg sqlcdb.UpdateOrganizationParams) error
	deleteOrganizationFn      func(ctx context.Context, id int64) error
	listOrgMembersFn          func(ctx context.Context, orgID int64) ([]sqlcdb.ListOrgMembersRow, error)
	updateOrgMemberRoleFn     func(ctx context.Context, arg sqlcdb.UpdateOrgMemberRoleParams) error
	countOrgAdminsFn          func(ctx context.Context, orgID int64) (int64, error)
	removeOrgMemberFn         func(ctx context.Context, arg sqlcdb.RemoveOrgMemberParams) error
	createOrgInviteFn         func(ctx context.Context, arg sqlcdb.CreateOrgInviteParams) (sqlcdb.OrgInvite, error)
	listOrgInvitesFn          func(ctx context.Context, orgID int64) ([]sqlcdb.ListOrgInvitesRow, error)
	deleteOrgInviteFn         func(ctx context.Context, arg sqlcdb.DeleteOrgInviteParams) error
	getOrgInviteByTokenFn     func(ctx context.Context, token string) (sqlcdb.GetOrgInviteByTokenRow, error)
	incrementInviteUseCountFn func(ctx context.Context, id int64) (int64, error)
	getOrgMembershipFn        func(ctx context.Context, arg sqlcdb.GetOrgMembershipParams) (sqlcdb.GetOrgMembershipRow, error)
	getOrgDashboardStatsFn    func(ctx context.Context, orgID types.NullInt64) (sqlcdb.GetOrgDashboardStatsRow, error)
	getOrgVoyagesByYearFn     func(ctx context.Context, orgID types.NullInt64) ([]sqlcdb.GetOrgVoyagesByYearRow, error)

	// cruises
	listCruisesFn  func(ctx context.Context, orgID int64) ([]sqlcdb.Cruise, error)
	countCruisesFn func(ctx context.Context, orgID int64) (int64, error)

	// pagination counts
	countTripsFn          func(ctx context.Context, ownerID int64) (int64, error)
	countOrgTripsFn       func(ctx context.Context, orgID types.NullInt64) (int64, error)
	countVoyagesFn        func(ctx context.Context, ownerID int64) (int64, error)
	countOrgVoyagesFn     func(ctx context.Context, orgID types.NullInt64) (int64, error)
	countYachtsFn         func(ctx context.Context, ownerID int64) (int64, error)
	countOrgYachtsFn      func(ctx context.Context, orgID types.NullInt64) (int64, error)
	countTrainingsFn      func(ctx context.Context, userID int64) (int64, error)
	countCrewMembersFn    func(ctx context.Context, ownerID int64) (int64, error)
	countOrgCrewMembersFn func(ctx context.Context, orgID types.NullInt64) (int64, error)

	// users
	getUserByIDFn      func(ctx context.Context, id int64) (sqlcdb.User, error)
	updateUserPatentFn func(ctx context.Context, arg sqlcdb.UpdateUserPatentParams) error
}

// --- trips ---

func (m *mockQuerier) ListTrips(ctx context.Context, arg sqlcdb.ListTripsParams) ([]sqlcdb.Trip, error) {
	if m.listTripsFn == nil {
		panic("unexpected call to ListTrips")
	}
	return m.listTripsFn(ctx, arg.OwnerID)
}

func (m *mockQuerier) CountTrips(ctx context.Context, ownerID int64) (int64, error) {
	if m.countTripsFn == nil {
		panic("unexpected call to CountTrips")
	}
	return m.countTripsFn(ctx, ownerID)
}

func (m *mockQuerier) GetTrip(ctx context.Context, arg sqlcdb.GetTripParams) (sqlcdb.Trip, error) {
	if m.getTripFn == nil {
		panic("unexpected call to GetTrip")
	}
	return m.getTripFn(ctx, arg)
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

func (m *mockQuerier) DeleteTrip(ctx context.Context, arg sqlcdb.DeleteTripParams) error {
	if m.deleteTripFn == nil {
		panic("unexpected call to DeleteTrip")
	}
	return m.deleteTripFn(ctx, arg)
}

func (m *mockQuerier) CancelTrip(ctx context.Context, arg sqlcdb.CancelTripParams) (sqlcdb.Trip, error) {
	if m.cancelTripFn == nil {
		panic("unexpected call to CancelTrip")
	}
	return m.cancelTripFn(ctx, arg)
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

func (m *mockQuerier) ClearTripEnrollToken(ctx context.Context, arg sqlcdb.ClearTripEnrollTokenParams) error {
	if m.clearTripEnrollTokenFn == nil {
		panic("unexpected call to ClearTripEnrollToken")
	}
	return m.clearTripEnrollTokenFn(ctx, arg)
}

func (m *mockQuerier) GetTripByEnrollToken(ctx context.Context, token types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
	if m.getTripByEnrollFn == nil {
		panic("unexpected call to GetTripByEnrollToken")
	}
	return m.getTripByEnrollFn(ctx, token)
}

// --- voyages ---

func (m *mockQuerier) ListVoyages(ctx context.Context, arg sqlcdb.ListVoyagesParams) ([]sqlcdb.Voyage, error) {
	if m.listVoyagesFn == nil {
		panic("unexpected call to ListVoyages")
	}
	return m.listVoyagesFn(ctx, arg.OwnerID)
}

func (m *mockQuerier) CountVoyages(ctx context.Context, ownerID int64) (int64, error) {
	if m.countVoyagesFn == nil {
		panic("unexpected call to CountVoyages")
	}
	return m.countVoyagesFn(ctx, ownerID)
}

func (m *mockQuerier) GetVoyage(ctx context.Context, arg sqlcdb.GetVoyageParams) (sqlcdb.Voyage, error) {
	if m.getVoyageFn == nil {
		panic("unexpected call to GetVoyage")
	}
	return m.getVoyageFn(ctx, arg)
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

func (m *mockQuerier) DeleteVoyage(ctx context.Context, arg sqlcdb.DeleteVoyageParams) error {
	if m.deleteVoyageFn == nil {
		panic("unexpected call to DeleteVoyage")
	}
	return m.deleteVoyageFn(ctx, arg)
}

func (m *mockQuerier) GetDashboardStats(ctx context.Context, ownerID int64) (sqlcdb.GetDashboardStatsRow, error) {
	if m.getDashboardFn == nil {
		panic("unexpected call to GetDashboardStats")
	}
	return m.getDashboardFn(ctx, ownerID)
}

func (m *mockQuerier) GetVoyagesByYear(ctx context.Context, ownerID int64) ([]sqlcdb.GetVoyagesByYearRow, error) {
	if m.getVoyagesByYearFn == nil {
		panic("unexpected call to GetVoyagesByYear")
	}
	return m.getVoyagesByYearFn(ctx, ownerID)
}

// --- org trips ---

func (m *mockQuerier) ListOrgTrips(ctx context.Context, arg sqlcdb.ListOrgTripsParams) ([]sqlcdb.Trip, error) {
	if m.listOrgTripsFn == nil {
		panic("unexpected call to ListOrgTrips")
	}
	return m.listOrgTripsFn(ctx, arg.OrgID)
}

func (m *mockQuerier) CountOrgTrips(ctx context.Context, orgID types.NullInt64) (int64, error) {
	if m.countOrgTripsFn == nil {
		panic("unexpected call to CountOrgTrips")
	}
	return m.countOrgTripsFn(ctx, orgID)
}

func (m *mockQuerier) GetOrgTrip(ctx context.Context, arg sqlcdb.GetOrgTripParams) (sqlcdb.Trip, error) {
	if m.getOrgTripFn == nil {
		panic("unexpected call to GetOrgTrip")
	}
	return m.getOrgTripFn(ctx, arg)
}

func (m *mockQuerier) CreateOrgTrip(ctx context.Context, arg sqlcdb.CreateOrgTripParams) (sqlcdb.Trip, error) {
	if m.createOrgTripFn == nil {
		panic("unexpected call to CreateOrgTrip")
	}
	return m.createOrgTripFn(ctx, arg)
}

func (m *mockQuerier) UpdateOrgTrip(ctx context.Context, arg sqlcdb.UpdateOrgTripParams) error {
	if m.updateOrgTripFn == nil {
		panic("unexpected call to UpdateOrgTrip")
	}
	return m.updateOrgTripFn(ctx, arg)
}

func (m *mockQuerier) DeleteOrgTrip(ctx context.Context, arg sqlcdb.DeleteOrgTripParams) error {
	if m.deleteOrgTripFn == nil {
		panic("unexpected call to DeleteOrgTrip")
	}
	return m.deleteOrgTripFn(ctx, arg)
}

func (m *mockQuerier) CancelOrgTrip(ctx context.Context, arg sqlcdb.CancelOrgTripParams) (sqlcdb.Trip, error) {
	if m.cancelOrgTripFn == nil {
		panic("unexpected call to CancelOrgTrip")
	}
	return m.cancelOrgTripFn(ctx, arg)
}

// --- org voyages ---

func (m *mockQuerier) ListOrgVoyages(ctx context.Context, arg sqlcdb.ListOrgVoyagesParams) ([]sqlcdb.Voyage, error) {
	if m.listOrgVoyagesFn == nil {
		panic("unexpected call to ListOrgVoyages")
	}
	return m.listOrgVoyagesFn(ctx, arg.OrgID)
}

func (m *mockQuerier) CountOrgVoyages(ctx context.Context, orgID types.NullInt64) (int64, error) {
	if m.countOrgVoyagesFn == nil {
		panic("unexpected call to CountOrgVoyages")
	}
	return m.countOrgVoyagesFn(ctx, orgID)
}

func (m *mockQuerier) GetOrgVoyage(ctx context.Context, arg sqlcdb.GetOrgVoyageParams) (sqlcdb.Voyage, error) {
	if m.getOrgVoyageFn == nil {
		panic("unexpected call to GetOrgVoyage")
	}
	return m.getOrgVoyageFn(ctx, arg)
}

func (m *mockQuerier) CreateOrgVoyage(ctx context.Context, arg sqlcdb.CreateOrgVoyageParams) (sqlcdb.Voyage, error) {
	if m.createOrgVoyageFn == nil {
		panic("unexpected call to CreateOrgVoyage")
	}
	return m.createOrgVoyageFn(ctx, arg)
}

func (m *mockQuerier) UpdateOrgVoyage(ctx context.Context, arg sqlcdb.UpdateOrgVoyageParams) error {
	if m.updateOrgVoyageFn == nil {
		panic("unexpected call to UpdateOrgVoyage")
	}
	return m.updateOrgVoyageFn(ctx, arg)
}

func (m *mockQuerier) DeleteOrgVoyage(ctx context.Context, arg sqlcdb.DeleteOrgVoyageParams) error {
	if m.deleteOrgVoyageFn == nil {
		panic("unexpected call to DeleteOrgVoyage")
	}
	return m.deleteOrgVoyageFn(ctx, arg)
}

// --- crew assignments ---

func (m *mockQuerier) CreateTripCrewAssignment(ctx context.Context, arg sqlcdb.CreateTripCrewAssignmentParams) (sqlcdb.CrewAssignment, error) {
	if m.createTripCrewFn == nil {
		panic("unexpected call to CreateTripCrewAssignment")
	}
	return m.createTripCrewFn(ctx, arg)
}

func (m *mockQuerier) ListTripCrewAssignments(ctx context.Context, arg sqlcdb.ListTripCrewAssignmentsParams) ([]sqlcdb.ListTripCrewAssignmentsRow, error) {
	if m.listTripCrewFn == nil {
		panic("unexpected call to ListTripCrewAssignments")
	}
	return m.listTripCrewFn(ctx, arg)
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

func (m *mockQuerier) ListVoyageCrewAssignments(ctx context.Context, arg sqlcdb.ListVoyageCrewAssignmentsParams) ([]sqlcdb.ListVoyageCrewAssignmentsRow, error) {
	if m.listVoyageCrewFn == nil {
		panic("unexpected call to ListVoyageCrewAssignments")
	}
	return m.listVoyageCrewFn(ctx, arg)
}

func (m *mockQuerier) DeleteVoyageCrewAssignment(ctx context.Context, arg sqlcdb.DeleteVoyageCrewAssignmentParams) error {
	if m.deleteVoyageCrewFn == nil {
		panic("unexpected call to DeleteVoyageCrewAssignment")
	}
	return m.deleteVoyageCrewFn(ctx, arg)
}

func (m *mockQuerier) ListOrgTripCrewAssignments(ctx context.Context, arg sqlcdb.ListOrgTripCrewAssignmentsParams) ([]sqlcdb.ListOrgTripCrewAssignmentsRow, error) {
	if m.listOrgTripCrewFn == nil {
		panic("unexpected call to ListOrgTripCrewAssignments")
	}
	return m.listOrgTripCrewFn(ctx, arg)
}

func (m *mockQuerier) DeleteOrgTripCrewAssignment(ctx context.Context, arg sqlcdb.DeleteOrgTripCrewAssignmentParams) error {
	if m.deleteOrgTripCrewFn == nil {
		panic("unexpected call to DeleteOrgTripCrewAssignment")
	}
	return m.deleteOrgTripCrewFn(ctx, arg)
}

func (m *mockQuerier) ListOrgVoyageCrewAssignments(ctx context.Context, arg sqlcdb.ListOrgVoyageCrewAssignmentsParams) ([]sqlcdb.ListOrgVoyageCrewAssignmentsRow, error) {
	if m.listOrgVoyageCrewFn == nil {
		panic("unexpected call to ListOrgVoyageCrewAssignments")
	}
	return m.listOrgVoyageCrewFn(ctx, arg)
}

func (m *mockQuerier) DeleteOrgVoyageCrewAssignment(ctx context.Context, arg sqlcdb.DeleteOrgVoyageCrewAssignmentParams) error {
	if m.deleteOrgVoyageCrewFn == nil {
		panic("unexpected call to DeleteOrgVoyageCrewAssignment")
	}
	return m.deleteOrgVoyageCrewFn(ctx, arg)
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

func (m *mockQuerier) ListTripEnrollments(ctx context.Context, arg sqlcdb.ListTripEnrollmentsParams) ([]sqlcdb.ListTripEnrollmentsRow, error) {
	if m.listTripEnrollmentsFn == nil {
		panic("unexpected call to ListTripEnrollments")
	}
	return m.listTripEnrollmentsFn(ctx, arg)
}

func (m *mockQuerier) UpdateTripEnrollmentStatus(ctx context.Context, arg sqlcdb.UpdateTripEnrollmentStatusParams) error {
	if m.updateTripEnrollmentStatusFn == nil {
		panic("unexpected call to UpdateTripEnrollmentStatus")
	}
	return m.updateTripEnrollmentStatusFn(ctx, arg)
}

func (m *mockQuerier) DeleteTripEnrollment(ctx context.Context, arg sqlcdb.DeleteTripEnrollmentParams) error {
	if m.deleteTripEnrollmentFn == nil {
		panic("unexpected call to DeleteTripEnrollment")
	}
	return m.deleteTripEnrollmentFn(ctx, arg)
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
	return m.listYachtsFn(ctx, arg.OwnerID)
}

func (m *mockQuerier) CountYachts(ctx context.Context, ownerID int64) (int64, error) {
	if m.countYachtsFn == nil {
		panic("unexpected call to CountYachts")
	}
	return m.countYachtsFn(ctx, ownerID)
}

func (m *mockQuerier) GetYacht(ctx context.Context, arg sqlcdb.GetYachtParams) (sqlcdb.Yacht, error) {
	if m.getYachtFn == nil {
		panic("unexpected call to GetYacht")
	}
	return m.getYachtFn(ctx, arg)
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

func (m *mockQuerier) DeleteYacht(ctx context.Context, arg sqlcdb.DeleteYachtParams) error {
	if m.deleteYachtFn == nil {
		panic("unexpected call to DeleteYacht")
	}
	return m.deleteYachtFn(ctx, arg)
}

func (m *mockQuerier) GetYachtByName(ctx context.Context, arg sqlcdb.GetYachtByNameParams) (sqlcdb.Yacht, error) {
	if m.getYachtByNameFn == nil {
		panic("unexpected call to GetYachtByName")
	}
	return m.getYachtByNameFn(ctx, arg)
}

func (m *mockQuerier) ListOrgYachts(ctx context.Context, arg sqlcdb.ListOrgYachtsParams) ([]sqlcdb.Yacht, error) {
	if m.listOrgYachtsFn == nil {
		panic("unexpected call to ListOrgYachts")
	}
	return m.listOrgYachtsFn(ctx, arg.OrgID)
}

func (m *mockQuerier) CountOrgYachts(ctx context.Context, orgID types.NullInt64) (int64, error) {
	if m.countOrgYachtsFn == nil {
		panic("unexpected call to CountOrgYachts")
	}
	return m.countOrgYachtsFn(ctx, orgID)
}

func (m *mockQuerier) GetOrgYacht(ctx context.Context, arg sqlcdb.GetOrgYachtParams) (sqlcdb.Yacht, error) {
	if m.getOrgYachtFn == nil {
		panic("unexpected call to GetOrgYacht")
	}
	return m.getOrgYachtFn(ctx, arg)
}

func (m *mockQuerier) CreateOrgYacht(ctx context.Context, arg sqlcdb.CreateOrgYachtParams) (sqlcdb.Yacht, error) {
	if m.createOrgYachtFn == nil {
		panic("unexpected call to CreateOrgYacht")
	}
	return m.createOrgYachtFn(ctx, arg)
}

func (m *mockQuerier) UpdateOrgYacht(ctx context.Context, arg sqlcdb.UpdateOrgYachtParams) error {
	if m.updateOrgYachtFn == nil {
		panic("unexpected call to UpdateOrgYacht")
	}
	return m.updateOrgYachtFn(ctx, arg)
}

func (m *mockQuerier) DeleteOrgYacht(ctx context.Context, arg sqlcdb.DeleteOrgYachtParams) error {
	if m.deleteOrgYachtFn == nil {
		panic("unexpected call to DeleteOrgYacht")
	}
	return m.deleteOrgYachtFn(ctx, arg)
}

// --- trainings ---

func (m *mockQuerier) ListTrainings(ctx context.Context, arg sqlcdb.ListTrainingsParams) ([]sqlcdb.Training, error) {
	if m.listTrainingsFn == nil {
		panic("unexpected call to ListTrainings")
	}
	return m.listTrainingsFn(ctx, arg.UserID)
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
	return m.listCrewMembersFn(ctx, arg.OwnerID)
}

func (m *mockQuerier) CountCrewMembers(ctx context.Context, ownerID int64) (int64, error) {
	if m.countCrewMembersFn == nil {
		panic("unexpected call to CountCrewMembers")
	}
	return m.countCrewMembersFn(ctx, ownerID)
}

func (m *mockQuerier) GetCrewMember(ctx context.Context, arg sqlcdb.GetCrewMemberParams) (sqlcdb.CrewMember, error) {
	if m.getCrewMemberFn == nil {
		panic("unexpected call to GetCrewMember")
	}
	return m.getCrewMemberFn(ctx, arg)
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

func (m *mockQuerier) DeleteCrewMember(ctx context.Context, arg sqlcdb.DeleteCrewMemberParams) error {
	if m.deleteCrewMemberFn == nil {
		panic("unexpected call to DeleteCrewMember")
	}
	return m.deleteCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) GetCrewMemberByName(ctx context.Context, arg sqlcdb.GetCrewMemberByNameParams) (sqlcdb.CrewMember, error) {
	if m.getCrewMemberByNameFn == nil {
		panic("unexpected call to GetCrewMemberByName")
	}
	return m.getCrewMemberByNameFn(ctx, arg)
}

func (m *mockQuerier) ListOrgCrewMembers(ctx context.Context, arg sqlcdb.ListOrgCrewMembersParams) ([]sqlcdb.CrewMember, error) {
	if m.listOrgCrewMembersFn == nil {
		panic("unexpected call to ListOrgCrewMembers")
	}
	return m.listOrgCrewMembersFn(ctx, arg.OrgID)
}

func (m *mockQuerier) CountOrgCrewMembers(ctx context.Context, orgID types.NullInt64) (int64, error) {
	if m.countOrgCrewMembersFn == nil {
		panic("unexpected call to CountOrgCrewMembers")
	}
	return m.countOrgCrewMembersFn(ctx, orgID)
}

func (m *mockQuerier) GetOrgCrewMember(ctx context.Context, arg sqlcdb.GetOrgCrewMemberParams) (sqlcdb.CrewMember, error) {
	if m.getOrgCrewMemberFn == nil {
		panic("unexpected call to GetOrgCrewMember")
	}
	return m.getOrgCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) CreateOrgCrewMember(ctx context.Context, arg sqlcdb.CreateOrgCrewMemberParams) (sqlcdb.CrewMember, error) {
	if m.createOrgCrewMemberFn == nil {
		panic("unexpected call to CreateOrgCrewMember")
	}
	return m.createOrgCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) UpdateOrgCrewMember(ctx context.Context, arg sqlcdb.UpdateOrgCrewMemberParams) error {
	if m.updateOrgCrewMemberFn == nil {
		panic("unexpected call to UpdateOrgCrewMember")
	}
	return m.updateOrgCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) DeleteOrgCrewMember(ctx context.Context, arg sqlcdb.DeleteOrgCrewMemberParams) error {
	if m.deleteOrgCrewMemberFn == nil {
		panic("unexpected call to DeleteOrgCrewMember")
	}
	return m.deleteOrgCrewMemberFn(ctx, arg)
}

// --- organizations ---

func (m *mockQuerier) ListUserOrganizations(ctx context.Context, userID int64) ([]sqlcdb.ListUserOrganizationsRow, error) {
	if m.listUserOrganizationsFn == nil {
		panic("unexpected call to ListUserOrganizations")
	}
	return m.listUserOrganizationsFn(ctx, userID)
}

func (m *mockQuerier) CreateOrganization(ctx context.Context, arg sqlcdb.CreateOrganizationParams) (sqlcdb.Organization, error) {
	if m.createOrganizationFn == nil {
		panic("unexpected call to CreateOrganization")
	}
	return m.createOrganizationFn(ctx, arg)
}

func (m *mockQuerier) AddOrgMember(ctx context.Context, arg sqlcdb.AddOrgMemberParams) (sqlcdb.OrgMember, error) {
	if m.addOrgMemberFn == nil {
		panic("unexpected call to AddOrgMember")
	}
	return m.addOrgMemberFn(ctx, arg)
}

func (m *mockQuerier) GetOrganizationBySlug(ctx context.Context, slug string) (sqlcdb.Organization, error) {
	if m.getOrganizationBySlugFn == nil {
		panic("unexpected call to GetOrganizationBySlug")
	}
	return m.getOrganizationBySlugFn(ctx, slug)
}

func (m *mockQuerier) UpdateOrganization(ctx context.Context, arg sqlcdb.UpdateOrganizationParams) error {
	if m.updateOrganizationFn == nil {
		panic("unexpected call to UpdateOrganization")
	}
	return m.updateOrganizationFn(ctx, arg)
}

func (m *mockQuerier) DeleteOrganization(ctx context.Context, id int64) error {
	if m.deleteOrganizationFn == nil {
		panic("unexpected call to DeleteOrganization")
	}
	return m.deleteOrganizationFn(ctx, id)
}

func (m *mockQuerier) ListOrgMembers(ctx context.Context, orgID int64) ([]sqlcdb.ListOrgMembersRow, error) {
	if m.listOrgMembersFn == nil {
		panic("unexpected call to ListOrgMembers")
	}
	return m.listOrgMembersFn(ctx, orgID)
}

func (m *mockQuerier) UpdateOrgMemberRole(ctx context.Context, arg sqlcdb.UpdateOrgMemberRoleParams) error {
	if m.updateOrgMemberRoleFn == nil {
		panic("unexpected call to UpdateOrgMemberRole")
	}
	return m.updateOrgMemberRoleFn(ctx, arg)
}

func (m *mockQuerier) CountOrgAdmins(ctx context.Context, orgID int64) (int64, error) {
	if m.countOrgAdminsFn == nil {
		panic("unexpected call to CountOrgAdmins")
	}
	return m.countOrgAdminsFn(ctx, orgID)
}

func (m *mockQuerier) RemoveOrgMember(ctx context.Context, arg sqlcdb.RemoveOrgMemberParams) error {
	if m.removeOrgMemberFn == nil {
		panic("unexpected call to RemoveOrgMember")
	}
	return m.removeOrgMemberFn(ctx, arg)
}

func (m *mockQuerier) CreateOrgInvite(ctx context.Context, arg sqlcdb.CreateOrgInviteParams) (sqlcdb.OrgInvite, error) {
	if m.createOrgInviteFn == nil {
		panic("unexpected call to CreateOrgInvite")
	}
	return m.createOrgInviteFn(ctx, arg)
}

func (m *mockQuerier) ListOrgInvites(ctx context.Context, orgID int64) ([]sqlcdb.ListOrgInvitesRow, error) {
	if m.listOrgInvitesFn == nil {
		panic("unexpected call to ListOrgInvites")
	}
	return m.listOrgInvitesFn(ctx, orgID)
}

func (m *mockQuerier) DeleteOrgInvite(ctx context.Context, arg sqlcdb.DeleteOrgInviteParams) error {
	if m.deleteOrgInviteFn == nil {
		panic("unexpected call to DeleteOrgInvite")
	}
	return m.deleteOrgInviteFn(ctx, arg)
}

func (m *mockQuerier) GetOrgInviteByToken(ctx context.Context, token string) (sqlcdb.GetOrgInviteByTokenRow, error) {
	if m.getOrgInviteByTokenFn == nil {
		panic("unexpected call to GetOrgInviteByToken")
	}
	return m.getOrgInviteByTokenFn(ctx, token)
}

func (m *mockQuerier) IncrementInviteUseCount(ctx context.Context, id int64) (int64, error) {
	if m.incrementInviteUseCountFn == nil {
		panic("unexpected call to IncrementInviteUseCount")
	}
	return m.incrementInviteUseCountFn(ctx, id)
}

func (m *mockQuerier) GetOrgMembership(ctx context.Context, arg sqlcdb.GetOrgMembershipParams) (sqlcdb.GetOrgMembershipRow, error) {
	if m.getOrgMembershipFn == nil {
		panic("unexpected call to GetOrgMembership")
	}
	return m.getOrgMembershipFn(ctx, arg)
}

func (m *mockQuerier) GetOrgDashboardStats(ctx context.Context, orgID types.NullInt64) (sqlcdb.GetOrgDashboardStatsRow, error) {
	if m.getOrgDashboardStatsFn == nil {
		panic("unexpected call to GetOrgDashboardStats")
	}
	return m.getOrgDashboardStatsFn(ctx, orgID)
}

func (m *mockQuerier) GetOrgVoyagesByYear(ctx context.Context, orgID types.NullInt64) ([]sqlcdb.GetOrgVoyagesByYearRow, error) {
	if m.getOrgVoyagesByYearFn == nil {
		panic("unexpected call to GetOrgVoyagesByYear")
	}
	return m.getOrgVoyagesByYearFn(ctx, orgID)
}

// --- panics for things we never set up in tests ---

func (m *mockQuerier) CreateUser(context.Context, sqlcdb.CreateUserParams) (sqlcdb.User, error) {
	panic("unexpected call to CreateUser")
}

func (m *mockQuerier) GetUserByEmail(context.Context, string) (sqlcdb.User, error) {
	panic("unexpected call to GetUserByEmail")
}

func (m *mockQuerier) GetUserByFirebaseUID(context.Context, types.NullString) (sqlcdb.User, error) {
	panic("unexpected call to GetUserByFirebaseUID")
}

func (m *mockQuerier) GetUserByID(ctx context.Context, id int64) (sqlcdb.User, error) {
	if m.getUserByIDFn == nil {
		panic("unexpected call to GetUserByID")
	}
	return m.getUserByIDFn(ctx, id)
}

func (m *mockQuerier) UpdateUserPatent(ctx context.Context, arg sqlcdb.UpdateUserPatentParams) error {
	if m.updateUserPatentFn == nil {
		panic("unexpected call to UpdateUserPatent")
	}
	return m.updateUserPatentFn(ctx, arg)
}

func (m *mockQuerier) LinkFirebaseUIDByEmail(context.Context, sqlcdb.LinkFirebaseUIDByEmailParams) (sqlcdb.User, error) {
	panic("unexpected call to LinkFirebaseUIDByEmail")
}

func (m *mockQuerier) UpdateUser(context.Context, sqlcdb.UpdateUserParams) error {
	panic("unexpected call to UpdateUser")
}

func (m *mockQuerier) UpsertUserByFirebaseUID(context.Context, sqlcdb.UpsertUserByFirebaseUIDParams) (sqlcdb.User, error) {
	panic("unexpected call to UpsertUserByFirebaseUID")
}

func (m *mockQuerier) GetOrganizationByID(context.Context, int64) (sqlcdb.Organization, error) {
	panic("unexpected call to GetOrganizationByID")
}

func (m *mockQuerier) GetOrgMembershipBySlug(context.Context, sqlcdb.GetOrgMembershipBySlugParams) (sqlcdb.OrgMember, error) {
	panic("unexpected call to GetOrgMembershipBySlug")
}

// --- org cruises (newer, no-fn stubs) ---

func (m *mockQuerier) ListCruises(ctx context.Context, arg sqlcdb.ListCruisesParams) ([]sqlcdb.Cruise, error) {
	if m.listCruisesFn == nil {
		panic("unexpected call to ListCruises")
	}
	return m.listCruisesFn(ctx, arg.OrgID)
}

func (m *mockQuerier) CountCruises(ctx context.Context, orgID int64) (int64, error) {
	if m.countCruisesFn == nil {
		panic("unexpected call to CountCruises")
	}
	return m.countCruisesFn(ctx, orgID)
}

func (m *mockQuerier) GetCruise(context.Context, sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
	panic("unexpected call to GetCruise")
}

func (m *mockQuerier) CreateCruise(context.Context, sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
	panic("unexpected call to CreateCruise")
}

func (m *mockQuerier) UpdateCruise(context.Context, sqlcdb.UpdateCruiseParams) error {
	panic("unexpected call to UpdateCruise")
}

func (m *mockQuerier) DeleteCruise(context.Context, sqlcdb.DeleteCruiseParams) error {
	panic("unexpected call to DeleteCruise")
}

func (m *mockQuerier) SetCruiseEnrollToken(context.Context, sqlcdb.SetCruiseEnrollTokenParams) error {
	panic("unexpected call to SetCruiseEnrollToken")
}

func (m *mockQuerier) ClearCruiseEnrollToken(context.Context, sqlcdb.ClearCruiseEnrollTokenParams) error {
	panic("unexpected call to ClearCruiseEnrollToken")
}

func (m *mockQuerier) GetCruiseByEnrollToken(ctx context.Context, token types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
	if m.getCruiseByEnrollFn == nil {
		panic("unexpected call to GetCruiseByEnrollToken")
	}
	return m.getCruiseByEnrollFn(ctx, token)
}

func (m *mockQuerier) ListCruiseTrips(context.Context, types.NullInt64) ([]sqlcdb.Trip, error) {
	panic("unexpected call to ListCruiseTrips")
}

func (m *mockQuerier) ListCruiseVoyages(context.Context, types.NullInt64) ([]sqlcdb.Voyage, error) {
	panic("unexpected call to ListCruiseVoyages")
}

func (m *mockQuerier) CreateCruiseEnrollment(context.Context, sqlcdb.CreateCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
	panic("unexpected call to CreateCruiseEnrollment")
}

func (m *mockQuerier) ListCruiseEnrollments(context.Context, sqlcdb.ListCruiseEnrollmentsParams) ([]sqlcdb.ListCruiseEnrollmentsRow, error) {
	panic("unexpected call to ListCruiseEnrollments")
}

func (m *mockQuerier) UpdateCruiseEnrollmentStatus(context.Context, sqlcdb.UpdateCruiseEnrollmentStatusParams) error {
	panic("unexpected call to UpdateCruiseEnrollmentStatus")
}

func (m *mockQuerier) AssignCruiseEnrollmentToTrip(context.Context, sqlcdb.AssignCruiseEnrollmentToTripParams) error {
	panic("unexpected call to AssignCruiseEnrollmentToTrip")
}

func (m *mockQuerier) DeleteCruiseEnrollment(context.Context, sqlcdb.DeleteCruiseEnrollmentParams) error {
	panic("unexpected call to DeleteCruiseEnrollment")
}

func (m *mockQuerier) CountCruiseEnrollments(context.Context, int64) (sqlcdb.CountCruiseEnrollmentsRow, error) {
	panic("unexpected call to CountCruiseEnrollments")
}

func (m *mockQuerier) GetUserCruiseEnrollment(context.Context, sqlcdb.GetUserCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
	panic("unexpected call to GetUserCruiseEnrollment")
}

func (m *mockQuerier) CreateVoyagePort(ctx context.Context, arg sqlcdb.CreateVoyagePortParams) (sqlcdb.VoyagePort, error) {
	if m.createVoyagePortFn == nil {
		panic("unexpected call to CreateVoyagePort")
	}
	return m.createVoyagePortFn(ctx, arg)
}

func (m *mockQuerier) ListVoyagePorts(ctx context.Context, arg sqlcdb.ListVoyagePortsParams) ([]sqlcdb.VoyagePort, error) {
	if m.listVoyagePortsFn == nil {
		panic("unexpected call to ListVoyagePorts")
	}
	return m.listVoyagePortsFn(ctx, arg)
}

func (m *mockQuerier) DeleteVoyagePort(ctx context.Context, arg sqlcdb.DeleteVoyagePortParams) error {
	if m.deleteVoyagePortFn == nil {
		panic("unexpected call to DeleteVoyagePort")
	}
	return m.deleteVoyagePortFn(ctx, arg)
}

func (m *mockQuerier) ListOrgVoyagePorts(ctx context.Context, arg sqlcdb.ListOrgVoyagePortsParams) ([]sqlcdb.VoyagePort, error) {
	if m.listOrgVoyagePortsFn == nil {
		panic("unexpected call to ListOrgVoyagePorts")
	}
	return m.listOrgVoyagePortsFn(ctx, arg)
}

func (m *mockQuerier) DeleteOrgVoyagePort(ctx context.Context, arg sqlcdb.DeleteOrgVoyagePortParams) error {
	if m.deleteOrgVoyagePortFn == nil {
		panic("unexpected call to DeleteOrgVoyagePort")
	}
	return m.deleteOrgVoyagePortFn(ctx, arg)
}

func userCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, middleware.UserCtxKey, &auth.Claims{
		UserID: 1, Email: "test@example.com", Name: "Test User",
	})
}
