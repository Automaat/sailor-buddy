// Named aliases over the generated OpenAPI schema. Components and stores import
// these instead of re-declaring DTO shapes, so any backend contract change
// surfaces as a compile error here rather than a runtime surprise.
import type { components } from './schema';

type Schemas = components['schemas'];

/** Page is the paginated list envelope returned by collection endpoints. */
export type Page<T> = Omit<Schemas['PageTrip'], 'items' | '$schema'> & { items: T[] };

export type User = Schemas['Me'];

export type Trip = Schemas['Trip'];
export type TripStatus = Trip['status'];
export type TripBody = Schemas['TripBody'];
export type Voyage = Schemas['Voyage'];
export type VoyageBody = Schemas['VoyageBody'];
export type CompleteTripPayload = Schemas['CompleteTripBody'];

export type VoyagePort = Schemas['VoyagePort'];
export type VoyagePortBody = Schemas['VoyagePortBody'];
export type GeocodeResult = Schemas['GeocodeResult'];

export type Yacht = Schemas['Yacht'];
export type YachtBody = Schemas['YachtBody'];

export type CrewMember = Schemas['CrewMember'];
export type CrewAssignment = Schemas['CrewAssignment'];

export type Training = Schemas['Training'];
export type TrainingBody = Schemas['TrainingBody'];

export type DashboardStats = Schemas['Dashboard'];
export type YearStats = Schemas['VoyagesByYear'];
export type OrgDashboardStats = Schemas['OrgDashboard'];

export type UploadResponse = Schemas['UploadURLOutputBody'];

export type TripEnrollment = Schemas['TripEnrollmentDetail'];
export type CruiseEnrollment = Schemas['CruiseEnrollmentDetail'];
export type Enrollment = Schemas['Enrollment'];

export type PublicTrip = Schemas['EnrollTrip'];
export type PublicCruise = Schemas['EnrollCruise'];

export type Cruise = Schemas['Cruise'];

export type VoyageOpinion = Schemas['VoyageOpinion'];

/** Organization carries the caller's role; returned by the org list endpoint. */
export type Organization = Schemas['UserOrganization'];
/** OrgDetail is the role-free organization record from the single-org endpoint. */
export type OrgDetail = Schemas['Organization'];
export type OrgMember = Schemas['OrgMember'];
export type OrgInvite = Schemas['OrgInvite'];
export type OrgInviteInfo = Schemas['InviteInfo'];

// EnrollInfo arrives flat (kind plus optional trip/cruise); the page treats it
// as a discriminated union, which the resolveEnroll route helper narrows to.
type EnrollBase = Omit<Schemas['EnrollInfo'], '$schema' | 'kind' | 'trip' | 'cruise' | 'trips'>;
export type EnrollPageData =
	| (EnrollBase & { kind: 'trip'; trip: PublicTrip })
	| (EnrollBase & { kind: 'cruise'; cruise: PublicCruise; trips: Trip[] | null });
