package handlers

import (
	"context"
	"database/sql"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type mockQuerier struct {
	listCruisesFn                        func(ctx context.Context, ownerID int64) ([]sqlcdb.Cruise, error)
	getCruiseFn                          func(ctx context.Context, arg sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error)
	createCruiseFn                       func(ctx context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error)
	updateCruiseFn                       func(ctx context.Context, arg sqlcdb.UpdateCruiseParams) error
	deleteCruiseFn                       func(ctx context.Context, arg sqlcdb.DeleteCruiseParams) error
	listYachtsFn                         func(ctx context.Context, ownerID int64) ([]sqlcdb.Yacht, error)
	getYachtFn                           func(ctx context.Context, arg sqlcdb.GetYachtParams) (sqlcdb.Yacht, error)
	createYachtFn                        func(ctx context.Context, arg sqlcdb.CreateYachtParams) (sqlcdb.Yacht, error)
	updateYachtFn                        func(ctx context.Context, arg sqlcdb.UpdateYachtParams) error
	deleteYachtFn                        func(ctx context.Context, arg sqlcdb.DeleteYachtParams) error
	listTrainingsFn                      func(ctx context.Context, userID int64) ([]sqlcdb.Training, error)
	getTrainingFn                        func(ctx context.Context, arg sqlcdb.GetTrainingParams) (sqlcdb.Training, error)
	createTrainingFn                     func(ctx context.Context, arg sqlcdb.CreateTrainingParams) (sqlcdb.Training, error)
	updateTrainingFn                     func(ctx context.Context, arg sqlcdb.UpdateTrainingParams) error
	deleteTrainingFn                     func(ctx context.Context, arg sqlcdb.DeleteTrainingParams) error
	listCrewMembersFn                    func(ctx context.Context, ownerID int64) ([]sqlcdb.CrewMember, error)
	getCrewMemberFn                      func(ctx context.Context, arg sqlcdb.GetCrewMemberParams) (sqlcdb.CrewMember, error)
	createCrewMemberFn                   func(ctx context.Context, arg sqlcdb.CreateCrewMemberParams) (sqlcdb.CrewMember, error)
	updateCrewMemberFn                   func(ctx context.Context, arg sqlcdb.UpdateCrewMemberParams) error
	deleteCrewMemberFn                   func(ctx context.Context, arg sqlcdb.DeleteCrewMemberParams) error
	createCrewAssignmentFn               func(ctx context.Context, arg sqlcdb.CreateCrewAssignmentParams) (sqlcdb.CrewAssignment, error)
	listCruiseCrewFn                     func(ctx context.Context, arg sqlcdb.ListCruiseCrewAssignmentsParams) ([]sqlcdb.ListCruiseCrewAssignmentsRow, error)
	deleteCrewAssignmentFn               func(ctx context.Context, arg sqlcdb.DeleteCrewAssignmentParams) error
	getDashboardStatsFn                  func(ctx context.Context, ownerID int64) (sqlcdb.GetDashboardStatsRow, error)
	getCruisesByYearFn                   func(ctx context.Context, ownerID int64) ([]sqlcdb.GetCruisesByYearRow, error)
	getYachtByNameFn                     func(ctx context.Context, arg sqlcdb.GetYachtByNameParams) (sqlcdb.Yacht, error)
	getCrewMemberByNameFn                func(ctx context.Context, arg sqlcdb.GetCrewMemberByNameParams) (sqlcdb.CrewMember, error)
	getCrewAssignmentByCruiseAndMemberFn func(ctx context.Context, arg sqlcdb.GetCrewAssignmentByCruiseAndMemberParams) (sqlcdb.GetCrewAssignmentByCruiseAndMemberRow, error)
	upsertVoyageOpinionFn                func(ctx context.Context, arg sqlcdb.UpsertVoyageOpinionParams) (sqlcdb.VoyageOpinion, error)
	listCruiseVoyageOpinionsFn           func(ctx context.Context, cruiseID int64) ([]sqlcdb.ListCruiseVoyageOpinionsRow, error)
	getVoyageOpinionFn                   func(ctx context.Context, id int64) (sqlcdb.VoyageOpinion, error)
	deleteVoyageOpinionFn                func(ctx context.Context, id int64) error
	getCruiseByEnrollTokenFn             func(ctx context.Context, token sql.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error)
	getUserEnrollmentFn                  func(ctx context.Context, arg sqlcdb.GetUserEnrollmentParams) (sqlcdb.CruiseEnrollment, error)
	countCruiseEnrollmentsFn             func(ctx context.Context, cruiseID int64) (sqlcdb.CountCruiseEnrollmentsRow, error)
	createCruiseEnrollmentFn             func(ctx context.Context, arg sqlcdb.CreateCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error)
	setCruiseEnrollTokenFn               func(ctx context.Context, arg sqlcdb.SetCruiseEnrollTokenParams) error
	clearCruiseEnrollTokenFn             func(ctx context.Context, arg sqlcdb.ClearCruiseEnrollTokenParams) error
	listCruiseEnrollmentsFn              func(ctx context.Context, arg sqlcdb.ListCruiseEnrollmentsParams) ([]sqlcdb.ListCruiseEnrollmentsRow, error)
	updateEnrollmentStatusFn             func(ctx context.Context, arg sqlcdb.UpdateEnrollmentStatusParams) error
	deleteCruiseEnrollmentFn             func(ctx context.Context, arg sqlcdb.DeleteCruiseEnrollmentParams) error
	// org methods
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
	listOrgYachtsFn           func(ctx context.Context, orgID sql.NullInt64) ([]sqlcdb.Yacht, error)
	getOrgYachtFn             func(ctx context.Context, arg sqlcdb.GetOrgYachtParams) (sqlcdb.Yacht, error)
	createOrgYachtFn          func(ctx context.Context, arg sqlcdb.CreateOrgYachtParams) (sqlcdb.Yacht, error)
	updateOrgYachtFn          func(ctx context.Context, arg sqlcdb.UpdateOrgYachtParams) error
	deleteOrgYachtFn          func(ctx context.Context, arg sqlcdb.DeleteOrgYachtParams) error
	listOrgCruisesFn          func(ctx context.Context, orgID sql.NullInt64) ([]sqlcdb.Cruise, error)
	getOrgCruiseFn            func(ctx context.Context, arg sqlcdb.GetOrgCruiseParams) (sqlcdb.Cruise, error)
	createOrgCruiseFn         func(ctx context.Context, arg sqlcdb.CreateOrgCruiseParams) (sqlcdb.Cruise, error)
	updateOrgCruiseFn         func(ctx context.Context, arg sqlcdb.UpdateOrgCruiseParams) error
	deleteOrgCruiseFn         func(ctx context.Context, arg sqlcdb.DeleteOrgCruiseParams) error
	listOrgCrewMembersFn      func(ctx context.Context, orgID sql.NullInt64) ([]sqlcdb.CrewMember, error)
	getOrgCrewMemberFn        func(ctx context.Context, arg sqlcdb.GetOrgCrewMemberParams) (sqlcdb.CrewMember, error)
	createOrgCrewMemberFn     func(ctx context.Context, arg sqlcdb.CreateOrgCrewMemberParams) (sqlcdb.CrewMember, error)
	updateOrgCrewMemberFn     func(ctx context.Context, arg sqlcdb.UpdateOrgCrewMemberParams) error
	deleteOrgCrewMemberFn     func(ctx context.Context, arg sqlcdb.DeleteOrgCrewMemberParams) error
	getOrgDashboardStatsFn    func(ctx context.Context, orgID sql.NullInt64) (sqlcdb.GetOrgDashboardStatsRow, error)
	getOrgCruisesByYearFn     func(ctx context.Context, orgID sql.NullInt64) ([]sqlcdb.GetOrgCruisesByYearRow, error)
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

func (m *mockQuerier) ClearCruiseEnrollToken(ctx context.Context, arg sqlcdb.ClearCruiseEnrollTokenParams) error {
	if m.clearCruiseEnrollTokenFn != nil {
		return m.clearCruiseEnrollTokenFn(ctx, arg)
	}
	panic("unexpected call to ClearCruiseEnrollToken")
}

func (m *mockQuerier) CountCruiseEnrollments(ctx context.Context, cruiseID int64) (sqlcdb.CountCruiseEnrollmentsRow, error) {
	if m.countCruiseEnrollmentsFn != nil {
		return m.countCruiseEnrollmentsFn(ctx, cruiseID)
	}
	panic("unexpected call to CountCruiseEnrollments")
}

func (m *mockQuerier) CreateCruiseEnrollment(ctx context.Context, arg sqlcdb.CreateCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
	if m.createCruiseEnrollmentFn != nil {
		return m.createCruiseEnrollmentFn(ctx, arg)
	}
	panic("unexpected call to CreateCruiseEnrollment")
}

func (m *mockQuerier) DeleteCruiseEnrollment(ctx context.Context, arg sqlcdb.DeleteCruiseEnrollmentParams) error {
	if m.deleteCruiseEnrollmentFn != nil {
		return m.deleteCruiseEnrollmentFn(ctx, arg)
	}
	panic("unexpected call to DeleteCruiseEnrollment")
}

func (m *mockQuerier) GetCruiseByEnrollToken(ctx context.Context, token sql.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
	if m.getCruiseByEnrollTokenFn != nil {
		return m.getCruiseByEnrollTokenFn(ctx, token)
	}
	panic("unexpected call to GetCruiseByEnrollToken")
}

func (m *mockQuerier) GetUserEnrollment(ctx context.Context, arg sqlcdb.GetUserEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
	if m.getUserEnrollmentFn != nil {
		return m.getUserEnrollmentFn(ctx, arg)
	}
	panic("unexpected call to GetUserEnrollment")
}

func (m *mockQuerier) ListCruiseEnrollments(ctx context.Context, arg sqlcdb.ListCruiseEnrollmentsParams) ([]sqlcdb.ListCruiseEnrollmentsRow, error) {
	if m.listCruiseEnrollmentsFn != nil {
		return m.listCruiseEnrollmentsFn(ctx, arg)
	}
	panic("unexpected call to ListCruiseEnrollments")
}

func (m *mockQuerier) SetCruiseEnrollToken(ctx context.Context, arg sqlcdb.SetCruiseEnrollTokenParams) error {
	if m.setCruiseEnrollTokenFn != nil {
		return m.setCruiseEnrollTokenFn(ctx, arg)
	}
	panic("unexpected call to SetCruiseEnrollToken")
}

func (m *mockQuerier) UpdateEnrollmentStatus(ctx context.Context, arg sqlcdb.UpdateEnrollmentStatusParams) error {
	if m.updateEnrollmentStatusFn != nil {
		return m.updateEnrollmentStatusFn(ctx, arg)
	}
	panic("unexpected call to UpdateEnrollmentStatus")
}

func (m *mockQuerier) AddOrgMember(ctx context.Context, arg sqlcdb.AddOrgMemberParams) (sqlcdb.OrgMember, error) {
	if m.addOrgMemberFn == nil {
		panic("unexpected call to AddOrgMember")
	}
	return m.addOrgMemberFn(ctx, arg)
}

func (m *mockQuerier) CountOrgAdmins(ctx context.Context, orgID int64) (int64, error) {
	if m.countOrgAdminsFn == nil {
		panic("unexpected call to CountOrgAdmins")
	}
	return m.countOrgAdminsFn(ctx, orgID)
}

func (m *mockQuerier) CreateOrgCrewMember(ctx context.Context, arg sqlcdb.CreateOrgCrewMemberParams) (sqlcdb.CrewMember, error) {
	if m.createOrgCrewMemberFn == nil {
		panic("unexpected call to CreateOrgCrewMember")
	}
	return m.createOrgCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) CreateOrgCruise(ctx context.Context, arg sqlcdb.CreateOrgCruiseParams) (sqlcdb.Cruise, error) {
	if m.createOrgCruiseFn == nil {
		panic("unexpected call to CreateOrgCruise")
	}
	return m.createOrgCruiseFn(ctx, arg)
}

func (m *mockQuerier) CreateOrgInvite(ctx context.Context, arg sqlcdb.CreateOrgInviteParams) (sqlcdb.OrgInvite, error) {
	if m.createOrgInviteFn == nil {
		panic("unexpected call to CreateOrgInvite")
	}
	return m.createOrgInviteFn(ctx, arg)
}

func (m *mockQuerier) CreateOrgYacht(ctx context.Context, arg sqlcdb.CreateOrgYachtParams) (sqlcdb.Yacht, error) {
	if m.createOrgYachtFn == nil {
		panic("unexpected call to CreateOrgYacht")
	}
	return m.createOrgYachtFn(ctx, arg)
}

func (m *mockQuerier) CreateOrganization(ctx context.Context, arg sqlcdb.CreateOrganizationParams) (sqlcdb.Organization, error) {
	if m.createOrganizationFn == nil {
		panic("unexpected call to CreateOrganization")
	}
	return m.createOrganizationFn(ctx, arg)
}

func (m *mockQuerier) DeleteOrgCrewMember(ctx context.Context, arg sqlcdb.DeleteOrgCrewMemberParams) error {
	if m.deleteOrgCrewMemberFn == nil {
		panic("unexpected call to DeleteOrgCrewMember")
	}
	return m.deleteOrgCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) DeleteOrgCruise(ctx context.Context, arg sqlcdb.DeleteOrgCruiseParams) error {
	if m.deleteOrgCruiseFn == nil {
		panic("unexpected call to DeleteOrgCruise")
	}
	return m.deleteOrgCruiseFn(ctx, arg)
}

func (m *mockQuerier) DeleteOrgInvite(ctx context.Context, arg sqlcdb.DeleteOrgInviteParams) error {
	if m.deleteOrgInviteFn == nil {
		panic("unexpected call to DeleteOrgInvite")
	}
	return m.deleteOrgInviteFn(ctx, arg)
}

func (m *mockQuerier) DeleteOrgYacht(ctx context.Context, arg sqlcdb.DeleteOrgYachtParams) error {
	if m.deleteOrgYachtFn == nil {
		panic("unexpected call to DeleteOrgYacht")
	}
	return m.deleteOrgYachtFn(ctx, arg)
}

func (m *mockQuerier) DeleteOrganization(ctx context.Context, id int64) error {
	if m.deleteOrganizationFn == nil {
		panic("unexpected call to DeleteOrganization")
	}
	return m.deleteOrganizationFn(ctx, id)
}

func (m *mockQuerier) GetOrgCrewMember(ctx context.Context, arg sqlcdb.GetOrgCrewMemberParams) (sqlcdb.CrewMember, error) {
	if m.getOrgCrewMemberFn == nil {
		panic("unexpected call to GetOrgCrewMember")
	}
	return m.getOrgCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) GetOrgCruise(ctx context.Context, arg sqlcdb.GetOrgCruiseParams) (sqlcdb.Cruise, error) {
	if m.getOrgCruiseFn == nil {
		panic("unexpected call to GetOrgCruise")
	}
	return m.getOrgCruiseFn(ctx, arg)
}

func (m *mockQuerier) GetOrgCruisesByYear(ctx context.Context, orgID sql.NullInt64) ([]sqlcdb.GetOrgCruisesByYearRow, error) {
	if m.getOrgCruisesByYearFn == nil {
		panic("unexpected call to GetOrgCruisesByYear")
	}
	return m.getOrgCruisesByYearFn(ctx, orgID)
}

func (m *mockQuerier) GetOrgDashboardStats(ctx context.Context, orgID sql.NullInt64) (sqlcdb.GetOrgDashboardStatsRow, error) {
	if m.getOrgDashboardStatsFn == nil {
		panic("unexpected call to GetOrgDashboardStats")
	}
	return m.getOrgDashboardStatsFn(ctx, orgID)
}

func (m *mockQuerier) GetOrgInviteByToken(ctx context.Context, token string) (sqlcdb.GetOrgInviteByTokenRow, error) {
	if m.getOrgInviteByTokenFn == nil {
		panic("unexpected call to GetOrgInviteByToken")
	}
	return m.getOrgInviteByTokenFn(ctx, token)
}

func (m *mockQuerier) GetOrgMembership(ctx context.Context, arg sqlcdb.GetOrgMembershipParams) (sqlcdb.GetOrgMembershipRow, error) {
	if m.getOrgMembershipFn == nil {
		panic("unexpected call to GetOrgMembership")
	}
	return m.getOrgMembershipFn(ctx, arg)
}

func (m *mockQuerier) GetOrgMembershipBySlug(context.Context, sqlcdb.GetOrgMembershipBySlugParams) (sqlcdb.OrgMember, error) {
	panic("unexpected call")
}

func (m *mockQuerier) GetOrgYacht(ctx context.Context, arg sqlcdb.GetOrgYachtParams) (sqlcdb.Yacht, error) {
	if m.getOrgYachtFn == nil {
		panic("unexpected call to GetOrgYacht")
	}
	return m.getOrgYachtFn(ctx, arg)
}

func (m *mockQuerier) GetOrganizationByID(context.Context, int64) (sqlcdb.Organization, error) {
	panic("unexpected call")
}

func (m *mockQuerier) GetOrganizationBySlug(ctx context.Context, slug string) (sqlcdb.Organization, error) {
	if m.getOrganizationBySlugFn == nil {
		panic("unexpected call to GetOrganizationBySlug")
	}
	return m.getOrganizationBySlugFn(ctx, slug)
}

func (m *mockQuerier) IncrementInviteUseCount(ctx context.Context, id int64) (int64, error) {
	if m.incrementInviteUseCountFn == nil {
		panic("unexpected call to IncrementInviteUseCount")
	}
	return m.incrementInviteUseCountFn(ctx, id)
}

func (m *mockQuerier) ListOrgCrewMembers(ctx context.Context, orgID sql.NullInt64) ([]sqlcdb.CrewMember, error) {
	if m.listOrgCrewMembersFn == nil {
		panic("unexpected call to ListOrgCrewMembers")
	}
	return m.listOrgCrewMembersFn(ctx, orgID)
}

func (m *mockQuerier) ListOrgCruises(ctx context.Context, orgID sql.NullInt64) ([]sqlcdb.Cruise, error) {
	if m.listOrgCruisesFn == nil {
		panic("unexpected call to ListOrgCruises")
	}
	return m.listOrgCruisesFn(ctx, orgID)
}

func (m *mockQuerier) ListOrgInvites(ctx context.Context, orgID int64) ([]sqlcdb.ListOrgInvitesRow, error) {
	if m.listOrgInvitesFn == nil {
		panic("unexpected call to ListOrgInvites")
	}
	return m.listOrgInvitesFn(ctx, orgID)
}

func (m *mockQuerier) ListOrgMembers(ctx context.Context, orgID int64) ([]sqlcdb.ListOrgMembersRow, error) {
	if m.listOrgMembersFn == nil {
		panic("unexpected call to ListOrgMembers")
	}
	return m.listOrgMembersFn(ctx, orgID)
}

func (m *mockQuerier) ListOrgYachts(ctx context.Context, orgID sql.NullInt64) ([]sqlcdb.Yacht, error) {
	if m.listOrgYachtsFn == nil {
		panic("unexpected call to ListOrgYachts")
	}
	return m.listOrgYachtsFn(ctx, orgID)
}

func (m *mockQuerier) ListUserOrganizations(ctx context.Context, userID int64) ([]sqlcdb.ListUserOrganizationsRow, error) {
	if m.listUserOrganizationsFn == nil {
		panic("unexpected call to ListUserOrganizations")
	}
	return m.listUserOrganizationsFn(ctx, userID)
}

func (m *mockQuerier) RemoveOrgMember(ctx context.Context, arg sqlcdb.RemoveOrgMemberParams) error {
	if m.removeOrgMemberFn == nil {
		panic("unexpected call to RemoveOrgMember")
	}
	return m.removeOrgMemberFn(ctx, arg)
}

func (m *mockQuerier) UpdateOrgCrewMember(ctx context.Context, arg sqlcdb.UpdateOrgCrewMemberParams) error {
	if m.updateOrgCrewMemberFn == nil {
		panic("unexpected call to UpdateOrgCrewMember")
	}
	return m.updateOrgCrewMemberFn(ctx, arg)
}

func (m *mockQuerier) UpdateOrgCruise(ctx context.Context, arg sqlcdb.UpdateOrgCruiseParams) error {
	if m.updateOrgCruiseFn == nil {
		panic("unexpected call to UpdateOrgCruise")
	}
	return m.updateOrgCruiseFn(ctx, arg)
}

func (m *mockQuerier) UpdateOrgMemberRole(ctx context.Context, arg sqlcdb.UpdateOrgMemberRoleParams) error {
	if m.updateOrgMemberRoleFn == nil {
		panic("unexpected call to UpdateOrgMemberRole")
	}
	return m.updateOrgMemberRoleFn(ctx, arg)
}

func (m *mockQuerier) UpdateOrgYacht(ctx context.Context, arg sqlcdb.UpdateOrgYachtParams) error {
	if m.updateOrgYachtFn == nil {
		panic("unexpected call to UpdateOrgYacht")
	}
	return m.updateOrgYachtFn(ctx, arg)
}

func (m *mockQuerier) UpdateOrganization(ctx context.Context, arg sqlcdb.UpdateOrganizationParams) error {
	if m.updateOrganizationFn == nil {
		panic("unexpected call to UpdateOrganization")
	}
	return m.updateOrganizationFn(ctx, arg)
}

func userCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, middleware.UserCtxKey, &auth.Claims{
		UserID: 1, Email: "test@example.com", Name: "Test User",
	})
}

func orgCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, middleware.OrgCtxKey, &middleware.OrgContext{
		OrgID: 1, Slug: "test-org", Role: "admin",
	})
}
