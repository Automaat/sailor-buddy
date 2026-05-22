// Typed route helpers. Pages call these instead of building URL strings, so
// every endpoint is checked against the generated OpenAPI `paths`. The app is
// a single club: all data is shared and routes are flat (no org prefix).
import { api } from './client';
import type { components } from './schema';
import type { EnrollPageData } from './types';

type Schemas = components['schemas'];

// ---- dashboard --------------------------------------------------------------

export function getDashboard() {
	return api.get('/dashboard');
}

// ---- trips ------------------------------------------------------------------

export function listTrips() {
	return api.list('/trips');
}

export function getTrip(id: number) {
	return api.get('/trips/{tripID}', { path: { tripID: id } });
}

export function createTrip(body: Schemas['TripBody']) {
	return api.post('/trips', { body });
}

export function updateTrip(id: number, body: Schemas['TripBody']) {
	return api.put('/trips/{tripID}', { path: { tripID: id }, body });
}

export function deleteTrip(id: number) {
	return api.del('/trips/{tripID}', { path: { tripID: id } });
}

export function cancelTrip(id: number) {
	return api.post('/trips/{tripID}/cancel', { path: { tripID: id } });
}

export function completeTrip(id: number, body: Schemas['CompleteTripBody']) {
	return api.post('/trips/{tripID}/complete', { path: { tripID: id }, body });
}

export function listTripCrew(id: number) {
	return api.get('/trips/{tripID}/crew', { path: { tripID: id } });
}

export function assignTripCrew(id: number, body: Schemas['CrewAssignmentBody']) {
	return api.post('/trips/{tripID}/crew', { path: { tripID: id }, body });
}

export function removeTripCrew(id: number, assignmentID: number) {
	return api.del('/trips/{tripID}/crew/{assignmentID}', { path: { tripID: id, assignmentID } });
}

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
	return api.list('/voyages');
}

export function getVoyage(id: number) {
	return api.get('/voyages/{voyageID}', { path: { voyageID: id } });
}

export function createVoyage(body: Schemas['VoyageBody']) {
	return api.post('/voyages', { body });
}

export function updateVoyage(id: number, body: Schemas['VoyageBody']) {
	return api.put('/voyages/{voyageID}', { path: { voyageID: id }, body });
}

export function deleteVoyage(id: number) {
	return api.del('/voyages/{voyageID}', { path: { voyageID: id } });
}

export function listVoyageCrew(id: number) {
	return api.get('/voyages/{voyageID}/crew', { path: { voyageID: id } });
}

export function assignVoyageCrew(id: number, body: Schemas['CrewAssignmentBody']) {
	return api.post('/voyages/{voyageID}/crew', { path: { voyageID: id }, body });
}

export function removeVoyageCrew(id: number, assignmentID: number) {
	return api.del('/voyages/{voyageID}/crew/{assignmentID}', {
		path: { voyageID: id, assignmentID }
	});
}

export function listVoyagePorts(id: number) {
	return api.get('/voyages/{voyageID}/ports', { path: { voyageID: id } });
}

export function addVoyagePort(id: number, body: Schemas['VoyagePortBody']) {
	return api.post('/voyages/{voyageID}/ports', { path: { voyageID: id }, body });
}

export function deleteVoyagePort(id: number, portID: number) {
	return api.del('/voyages/{voyageID}/ports/{portID}', { path: { voyageID: id, portID } });
}

// reorderVoyagePorts persists a new visit order; portIDs is the full list in
// the desired sequence. Returns the ports with their updated positions.
export function reorderVoyagePorts(id: number, portIDs: number[]) {
	return api.put('/voyages/{voyageID}/ports/order', {
		path: { voyageID: id },
		body: { port_ids: portIDs }
	});
}

// geocode searches towns/places by name via the backend Nominatim proxy.
export function geocode(q: string): Promise<Schemas['GeocodeResult'][]> {
	return api.get('/geocode', { query: { q } }).then((r) => r ?? []);
}

export function listVoyageOpinions(id: number) {
	return api.get('/voyages/{voyageID}/opinions', { path: { voyageID: id } });
}

export function generateVoyageOpinion(id: number, body: Schemas['GenerateOpinionBody']) {
	return api.post('/voyages/{voyageID}/opinions', { path: { voyageID: id }, body });
}

export function deleteVoyageOpinion(id: number, opinionID: number) {
	return api.del('/voyages/{voyageID}/opinions/{opinionID}', {
		path: { voyageID: id, opinionID }
	});
}

export function downloadVoyageOpinion(id: number, opinionID: number) {
	return api.download('/voyages/{voyageID}/opinions/{opinionID}/download', {
		path: { voyageID: id, opinionID }
	});
}

// ---- yachts -----------------------------------------------------------------

export function listYachts() {
	return api.list('/yachts');
}

export function createYacht(body: Schemas['YachtBody']) {
	return api.post('/yachts', { body });
}

export function deleteYacht(id: number) {
	return api.del('/yachts/{yachtID}', { path: { yachtID: id } });
}

// ---- crew -------------------------------------------------------------------

export function listCrew() {
	return api.list('/crew');
}

// listAssignableCrew returns the crew pool for trip/voyage assignment pickers.
export function listAssignableCrew() {
	return api.list('/crew');
}

export function getCrew(id: number) {
	return api.get('/crew/{crewID}', { path: { crewID: id } });
}

export function createCrew(body: Schemas['CrewMemberBody']) {
	return api.post('/crew', { body });
}

export function deleteCrew(id: number) {
	return api.del('/crew/{crewID}', { path: { crewID: id } });
}

// ---- trainings (per-member) -------------------------------------------------

export function listTrainings() {
	return api.list('/trainings');
}

export function createTraining(body: Schemas['TrainingBody']) {
	return api.post('/trainings', { body });
}

export function deleteTraining(id: number) {
	return api.del('/trainings/{trainingID}', { path: { trainingID: id } });
}

// ---- cruises ----------------------------------------------------------------

export function listCruises(): Promise<Schemas['Cruise'][]> {
	return api.list('/cruises');
}

export function getCruise(id: number): Promise<Schemas['Cruise'] | null> {
	return api.get('/cruises/{cruiseID}', { path: { cruiseID: id } });
}

export function createCruise(body: Schemas['CruiseBody']) {
	return api.post('/cruises', { body });
}

export function updateCruise(id: number, body: Schemas['CruiseBody']) {
	return api.put('/cruises/{cruiseID}', { path: { cruiseID: id }, body });
}

export function deleteCruise(id: number) {
	return api.del('/cruises/{cruiseID}', { path: { cruiseID: id } });
}

export function listCruiseTrips(id: number) {
	return api.get('/cruises/{cruiseID}/trips', { path: { cruiseID: id } });
}

export function listCruiseVoyages(id: number) {
	return api.get('/cruises/{cruiseID}/voyages', { path: { cruiseID: id } });
}

export function listCruiseEnrollments(id: number) {
	return api.get('/cruises/{cruiseID}/enrollments', { path: { cruiseID: id } });
}

export function generateCruiseEnrollToken(id: number) {
	return api.post('/cruises/{cruiseID}/enroll-token', { path: { cruiseID: id } });
}

export function clearCruiseEnrollToken(id: number) {
	return api.del('/cruises/{cruiseID}/enroll-token', { path: { cruiseID: id } });
}

export function updateCruiseEnrollmentStatus(
	cruiseID: number,
	enrollmentID: number,
	status: Schemas['EnrollmentStatusBody']['status']
) {
	return api.put('/cruises/{cruiseID}/enrollments/{enrollmentID}/status', {
		path: { cruiseID, enrollmentID },
		body: { status }
	});
}

export function assignCruiseEnrollmentTrip(
	cruiseID: number,
	enrollmentID: number,
	tripID: number | undefined
) {
	return api.put('/cruises/{cruiseID}/enrollments/{enrollmentID}/trip', {
		path: { cruiseID, enrollmentID },
		body: { trip_id: tripID }
	});
}

export function deleteCruiseEnrollment(cruiseID: number, enrollmentID: number) {
	return api.del('/cruises/{cruiseID}/enrollments/{enrollmentID}', {
		path: { cruiseID, enrollmentID }
	});
}

// ---- members ----------------------------------------------------------------

export function listMembers() {
	return api.get('/members');
}

export function updateMemberRole(userID: number, role: Schemas['RoleBody']['role']) {
	return api.put('/members/{userID}/role', { path: { userID }, body: { role } });
}

// ---- account, enrollment, import -------------------------------------------

export function getMe() {
	return api.get('/auth/me');
}

export function updateMe(body: Schemas['UpdatePatentBody']) {
	return api.put('/auth/me', { body });
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

export function importXlsx(formData: FormData) {
	return api.upload('/import/xlsx', formData);
}

export function importConfirm(body: Schemas['ImportData']) {
	return api.post('/import/confirm', { body });
}

export function uploadImage(formData: FormData) {
	return api.upload('/upload/image', formData);
}
