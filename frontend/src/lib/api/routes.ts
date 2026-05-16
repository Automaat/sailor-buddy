// Typed route helpers. Pages call these instead of building URL strings, so
// the personal/organization path split lives in one place and every endpoint
// is checked against the generated OpenAPI `paths`.
import { api } from './client';
import { orgStore } from '$lib/stores/org.svelte';
import type { components } from './schema';
import type { EnrollPageData } from './types';

type Schemas = components['schemas'];

// requireOrg returns the active org slug or throws — used by helpers for
// endpoints that only exist in organization context.
function requireOrg(): string {
	const slug = orgStore.currentSlug;
	if (!slug) throw new Error('Brak kontekstu organizacji');
	return slug;
}

// ---- dashboard --------------------------------------------------------------

export function getDashboard() {
	const slug = orgStore.currentSlug;
	return slug
		? api.get('/orgs/{slug}/dashboard', { path: { slug } })
		: api.get('/dashboard');
}

// ---- trips ------------------------------------------------------------------

export function listTrips() {
	const slug = orgStore.currentSlug;
	return slug ? api.list('/orgs/{slug}/trips', { path: { slug } }) : api.list('/trips');
}

export function getTrip(id: number) {
	const slug = orgStore.currentSlug;
	return slug
		? api.get('/orgs/{slug}/trips/{tripID}', { path: { slug, tripID: id } })
		: api.get('/trips/{tripID}', { path: { tripID: id } });
}

export function createTrip(body: Schemas['TripBody']) {
	const slug = orgStore.currentSlug;
	return slug
		? api.post('/orgs/{slug}/trips', { path: { slug }, body })
		: api.post('/trips', { body });
}

export function updateTrip(id: number, body: Schemas['TripBody']) {
	const slug = orgStore.currentSlug;
	return slug
		? api.put('/orgs/{slug}/trips/{tripID}', { path: { slug, tripID: id }, body })
		: api.put('/trips/{tripID}', { path: { tripID: id }, body });
}

export function deleteTrip(id: number) {
	const slug = orgStore.currentSlug;
	return slug
		? api.del('/orgs/{slug}/trips/{tripID}', { path: { slug, tripID: id } })
		: api.del('/trips/{tripID}', { path: { tripID: id } });
}

export function cancelTrip(id: number) {
	const slug = orgStore.currentSlug;
	return slug
		? api.post('/orgs/{slug}/trips/{tripID}/cancel', { path: { slug, tripID: id } })
		: api.post('/trips/{tripID}/cancel', { path: { tripID: id } });
}

export function completeTrip(id: number, body: Schemas['CompleteTripBody']) {
	const slug = orgStore.currentSlug;
	return slug
		? api.post('/orgs/{slug}/trips/{tripID}/complete', { path: { slug, tripID: id }, body })
		: api.post('/trips/{tripID}/complete', { path: { tripID: id }, body });
}

export function listTripCrew(id: number) {
	const slug = orgStore.currentSlug;
	return slug
		? api.get('/orgs/{slug}/trips/{tripID}/crew', { path: { slug, tripID: id } })
		: api.get('/trips/{tripID}/crew', { path: { tripID: id } });
}

export function assignTripCrew(id: number, body: Schemas['CrewAssignmentBody']) {
	const slug = orgStore.currentSlug;
	return slug
		? api.post('/orgs/{slug}/trips/{tripID}/crew', { path: { slug, tripID: id }, body })
		: api.post('/trips/{tripID}/crew', { path: { tripID: id }, body });
}

export function removeTripCrew(id: number, assignmentID: number) {
	const slug = orgStore.currentSlug;
	return slug
		? api.del('/orgs/{slug}/trips/{tripID}/crew/{assignmentID}', {
				path: { slug, tripID: id, assignmentID }
			})
		: api.del('/trips/{tripID}/crew/{assignmentID}', { path: { tripID: id, assignmentID } });
}

// Trip enroll tokens and enrollments exist only for personal trips; org trips
// take their crew through cruise enrollments instead.
export function generateTripEnrollToken(id: number) {
	return api.post('/trips/{tripID}/enroll-token', { path: { tripID: id } });
}

export function clearTripEnrollToken(id: number) {
	return api.del('/trips/{tripID}/enroll-token', { path: { tripID: id } });
}

export function listTripEnrollments(id: number) {
	return api.get('/trips/{tripID}/enrollments', { path: { tripID: id } });
}

export function updateTripEnrollmentStatus(
	tripID: number,
	enrollmentID: number,
	status: Schemas['EnrollmentStatusBody']['status']
) {
	return api.put('/trips/{tripID}/enrollments/{id}/status', {
		path: { tripID, id: enrollmentID },
		body: { status }
	});
}

export function deleteTripEnrollment(tripID: number, enrollmentID: number) {
	return api.del('/trips/{tripID}/enrollments/{id}', { path: { tripID, id: enrollmentID } });
}

// ---- voyages ----------------------------------------------------------------

export function listVoyages() {
	const slug = orgStore.currentSlug;
	return slug ? api.list('/orgs/{slug}/voyages', { path: { slug } }) : api.list('/voyages');
}

export function getVoyage(id: number) {
	const slug = orgStore.currentSlug;
	return slug
		? api.get('/orgs/{slug}/voyages/{voyageID}', { path: { slug, voyageID: id } })
		: api.get('/voyages/{voyageID}', { path: { voyageID: id } });
}

export function createVoyage(body: Schemas['VoyageBody']) {
	const slug = orgStore.currentSlug;
	return slug
		? api.post('/orgs/{slug}/voyages', { path: { slug }, body })
		: api.post('/voyages', { body });
}

export function updateVoyage(id: number, body: Schemas['VoyageBody']) {
	const slug = orgStore.currentSlug;
	return slug
		? api.put('/orgs/{slug}/voyages/{voyageID}', { path: { slug, voyageID: id }, body })
		: api.put('/voyages/{voyageID}', { path: { voyageID: id }, body });
}

export function deleteVoyage(id: number) {
	const slug = orgStore.currentSlug;
	return slug
		? api.del('/orgs/{slug}/voyages/{voyageID}', { path: { slug, voyageID: id } })
		: api.del('/voyages/{voyageID}', { path: { voyageID: id } });
}

export function listVoyageCrew(id: number) {
	const slug = orgStore.currentSlug;
	return slug
		? api.get('/orgs/{slug}/voyages/{voyageID}/crew', { path: { slug, voyageID: id } })
		: api.get('/voyages/{voyageID}/crew', { path: { voyageID: id } });
}

export function assignVoyageCrew(id: number, body: Schemas['CrewAssignmentBody']) {
	const slug = orgStore.currentSlug;
	return slug
		? api.post('/orgs/{slug}/voyages/{voyageID}/crew', { path: { slug, voyageID: id }, body })
		: api.post('/voyages/{voyageID}/crew', { path: { voyageID: id }, body });
}

export function removeVoyageCrew(id: number, assignmentID: number) {
	const slug = orgStore.currentSlug;
	return slug
		? api.del('/orgs/{slug}/voyages/{voyageID}/crew/{assignmentID}', {
				path: { slug, voyageID: id, assignmentID }
			})
		: api.del('/voyages/{voyageID}/crew/{assignmentID}', {
				path: { voyageID: id, assignmentID }
			});
}

export function listVoyageOpinions(id: number) {
	const slug = orgStore.currentSlug;
	return slug
		? api.get('/orgs/{slug}/voyages/{voyageID}/opinions', { path: { slug, voyageID: id } })
		: api.get('/voyages/{voyageID}/opinions', { path: { voyageID: id } });
}

export function generateVoyageOpinion(id: number, body: Schemas['GenerateOpinionBody']) {
	const slug = orgStore.currentSlug;
	return slug
		? api.post('/orgs/{slug}/voyages/{voyageID}/opinions', {
				path: { slug, voyageID: id },
				body
			})
		: api.post('/voyages/{voyageID}/opinions', { path: { voyageID: id }, body });
}

export function deleteVoyageOpinion(id: number, opinionID: number) {
	const slug = orgStore.currentSlug;
	return slug
		? api.del('/orgs/{slug}/voyages/{voyageID}/opinions/{opinionID}', {
				path: { slug, voyageID: id, opinionID }
			})
		: api.del('/voyages/{voyageID}/opinions/{opinionID}', {
				path: { voyageID: id, opinionID }
			});
}

export function downloadVoyageOpinion(id: number, opinionID: number) {
	const slug = orgStore.currentSlug;
	return slug
		? api.download('/orgs/{slug}/voyages/{voyageID}/opinions/{opinionID}/download', {
				path: { slug, voyageID: id, opinionID }
			})
		: api.download('/voyages/{voyageID}/opinions/{opinionID}/download', {
				path: { voyageID: id, opinionID }
			});
}

// ---- yachts -----------------------------------------------------------------

export function listYachts() {
	const slug = orgStore.currentSlug;
	return slug ? api.list('/orgs/{slug}/yachts', { path: { slug } }) : api.list('/yachts');
}

export function createYacht(body: Schemas['YachtBody']) {
	const slug = orgStore.currentSlug;
	return slug
		? api.post('/orgs/{slug}/yachts', { path: { slug }, body })
		: api.post('/yachts', { body });
}

export function deleteYacht(id: number) {
	const slug = orgStore.currentSlug;
	return slug
		? api.del('/orgs/{slug}/yachts/{yachtID}', { path: { slug, yachtID: id } })
		: api.del('/yachts/{yachtID}', { path: { yachtID: id } });
}

// ---- crew -------------------------------------------------------------------

export function listCrew() {
	const slug = orgStore.currentSlug;
	return slug ? api.list('/orgs/{slug}/crew', { path: { slug } }) : api.list('/crew');
}

export function getCrew(id: number) {
	const slug = orgStore.currentSlug;
	return slug
		? api.get('/orgs/{slug}/crew/{crewID}', { path: { slug, crewID: id } })
		: api.get('/crew/{crewID}', { path: { crewID: id } });
}

export function createCrew(body: Schemas['CrewMemberBody']) {
	const slug = orgStore.currentSlug;
	return slug
		? api.post('/orgs/{slug}/crew', { path: { slug }, body })
		: api.post('/crew', { body });
}

export function deleteCrew(id: number) {
	const slug = orgStore.currentSlug;
	return slug
		? api.del('/orgs/{slug}/crew/{crewID}', { path: { slug, crewID: id } })
		: api.del('/crew/{crewID}', { path: { crewID: id } });
}

// ---- trainings (personal only) ---------------------------------------------

export function listTrainings() {
	return api.list('/trainings');
}

export function createTraining(body: Schemas['TrainingBody']) {
	return api.post('/trainings', { body });
}

export function deleteTraining(id: number) {
	return api.del('/trainings/{trainingID}', { path: { trainingID: id } });
}

// ---- cruises (organization only) -------------------------------------------

// listCruises and getCruise degrade to empty/null outside org context so the
// personal trip/voyage pages, which have no cruise concept, stay simple.
export function listCruises(): Promise<Schemas['Cruise'][]> {
	const slug = orgStore.currentSlug;
	return slug ? api.list('/orgs/{slug}/cruises', { path: { slug } }) : Promise.resolve([]);
}

export function getCruise(id: number): Promise<Schemas['Cruise'] | null> {
	const slug = orgStore.currentSlug;
	return slug
		? api.get('/orgs/{slug}/cruises/{cruiseID}', { path: { slug, cruiseID: id } })
		: Promise.resolve(null);
}

export function createCruise(body: Schemas['CruiseBody']) {
	return api.post('/orgs/{slug}/cruises', { path: { slug: requireOrg() }, body });
}

export function updateCruise(id: number, body: Schemas['CruiseBody']) {
	return api.put('/orgs/{slug}/cruises/{cruiseID}', {
		path: { slug: requireOrg(), cruiseID: id },
		body
	});
}

export function deleteCruise(id: number) {
	return api.del('/orgs/{slug}/cruises/{cruiseID}', {
		path: { slug: requireOrg(), cruiseID: id }
	});
}

export function listCruiseTrips(id: number) {
	return api.get('/orgs/{slug}/cruises/{cruiseID}/trips', {
		path: { slug: requireOrg(), cruiseID: id }
	});
}

export function listCruiseVoyages(id: number) {
	return api.get('/orgs/{slug}/cruises/{cruiseID}/voyages', {
		path: { slug: requireOrg(), cruiseID: id }
	});
}

export function listCruiseEnrollments(id: number) {
	return api.get('/orgs/{slug}/cruises/{cruiseID}/enrollments', {
		path: { slug: requireOrg(), cruiseID: id }
	});
}

export function generateCruiseEnrollToken(id: number) {
	return api.post('/orgs/{slug}/cruises/{cruiseID}/enroll-token', {
		path: { slug: requireOrg(), cruiseID: id }
	});
}

export function clearCruiseEnrollToken(id: number) {
	return api.del('/orgs/{slug}/cruises/{cruiseID}/enroll-token', {
		path: { slug: requireOrg(), cruiseID: id }
	});
}

export function updateCruiseEnrollmentStatus(
	cruiseID: number,
	enrollmentID: number,
	status: Schemas['EnrollmentStatusBody']['status']
) {
	return api.put('/orgs/{slug}/cruises/{cruiseID}/enrollments/{enrollmentID}/status', {
		path: { slug: requireOrg(), cruiseID, enrollmentID },
		body: { status }
	});
}

export function assignCruiseEnrollmentTrip(
	cruiseID: number,
	enrollmentID: number,
	tripID: number | undefined
) {
	return api.put('/orgs/{slug}/cruises/{cruiseID}/enrollments/{enrollmentID}/trip', {
		path: { slug: requireOrg(), cruiseID, enrollmentID },
		body: { trip_id: tripID }
	});
}

export function deleteCruiseEnrollment(cruiseID: number, enrollmentID: number) {
	return api.del('/orgs/{slug}/cruises/{cruiseID}/enrollments/{enrollmentID}', {
		path: { slug: requireOrg(), cruiseID, enrollmentID }
	});
}

// ---- organizations ----------------------------------------------------------

export function listOrgs() {
	return api.get('/orgs');
}

export function getOrg(slug: string) {
	return api.get('/orgs/{slug}', { path: { slug } });
}

export function createOrg(body: Schemas['OrgBody']) {
	return api.post('/orgs', { body });
}

export function updateOrg(slug: string, body: Schemas['OrgBody']) {
	return api.put('/orgs/{slug}', { path: { slug }, body });
}

export function deleteOrg(slug: string) {
	return api.del('/orgs/{slug}', { path: { slug } });
}

export function listOrgMembers(slug: string) {
	return api.get('/orgs/{slug}/members', { path: { slug } });
}

export function removeOrgMember(slug: string, memberID: number) {
	return api.del('/orgs/{slug}/members/{memberID}', { path: { slug, memberID } });
}

export function updateOrgMemberRole(
	slug: string,
	memberID: number,
	role: Schemas['MemberRoleBody']['role']
) {
	return api.put('/orgs/{slug}/members/{memberID}/role', {
		path: { slug, memberID },
		body: { role }
	});
}

export function listOrgInvites(slug: string) {
	return api.get('/orgs/{slug}/invites', { path: { slug } });
}

export function createOrgInvite(slug: string, body: Schemas['InviteRequestBody']) {
	return api.post('/orgs/{slug}/invites', { path: { slug }, body });
}

export function deleteOrgInvite(slug: string, inviteID: number) {
	return api.del('/orgs/{slug}/invites/{inviteID}', { path: { slug, inviteID } });
}

// ---- account, enrollment, invites, import ----------------------------------

export function getMe() {
	return api.get('/auth/me');
}

// resolveEnroll rebuilds the flat EnrollInfo response into the discriminated
// union the page consumes. The backend populates trip/cruise per `kind`; this
// throws rather than emitting a malformed union when that contract is broken.
export async function resolveEnroll(token: string): Promise<EnrollPageData> {
	const info = await api.get('/enroll/{token}', { path: { token } });
	if (info.kind === 'cruise') {
		if (!info.cruise) throw new Error('Brak danych wydarzenia');
		return { ...info, kind: 'cruise', cruise: info.cruise, trips: info.trips ?? null };
	}
	if (!info.trip) throw new Error('Brak danych rejsu');
	return { ...info, kind: 'trip', trip: info.trip };
}

export function enroll(token: string, note: string | undefined) {
	return api.post('/enroll/{token}', { path: { token }, body: { note } });
}

export function getInviteInfo(token: string) {
	return api.get('/join/{token}', { path: { token } });
}

export function acceptInvite(token: string) {
	return api.post('/join/{token}', { path: { token } });
}

export function importXlsx(formData: FormData) {
	return api.upload('/import/xlsx', formData);
}

export function importConfirm(body: Schemas['ImportData']) {
	return api.post('/import/confirm', { body });
}

export function uploadImage(formData: FormData) {
	return api.upload('/upload/image', formData);
}
