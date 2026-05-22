import { describe, it, expect, beforeEach, vi } from 'vitest';

// The route helpers are thin typed wrappers over the `api` client; mock the
// client so each helper can be checked for the verb, path and params it sends.
const api = vi.hoisted(() => ({
	get: vi.fn(),
	list: vi.fn(),
	post: vi.fn(),
	put: vi.fn(),
	del: vi.fn(),
	download: vi.fn(),
	upload: vi.fn()
}));

vi.mock('./client', () => ({ api }));

import * as routes from './routes';

type Verb = keyof typeof api;

beforeEach(() => {
	for (const fn of Object.values(api)) fn.mockReset().mockResolvedValue(undefined);
	api.list.mockResolvedValue([]);
	api.get.mockResolvedValue({});
});

const body = { name: 'X' } as never;
const fd = new FormData();

// Each case asserts a helper delegates to one api verb with exact arguments.
const cases: Array<{ name: string; run: () => unknown; verb: Verb; args: unknown[] }> = [
	{ name: 'getDashboard', run: () => routes.getDashboard(), verb: 'get', args: ['/dashboard'] },

	{ name: 'listTrips', run: () => routes.listTrips(), verb: 'list', args: ['/trips'] },
	{
		name: 'getTrip',
		run: () => routes.getTrip(5),
		verb: 'get',
		args: ['/trips/{tripID}', { path: { tripID: 5 } }]
	},
	{ name: 'createTrip', run: () => routes.createTrip(body), verb: 'post', args: ['/trips', { body }] },
	{
		name: 'updateTrip',
		run: () => routes.updateTrip(5, body),
		verb: 'put',
		args: ['/trips/{tripID}', { path: { tripID: 5 }, body }]
	},
	{
		name: 'deleteTrip',
		run: () => routes.deleteTrip(5),
		verb: 'del',
		args: ['/trips/{tripID}', { path: { tripID: 5 } }]
	},
	{
		name: 'cancelTrip',
		run: () => routes.cancelTrip(5),
		verb: 'post',
		args: ['/trips/{tripID}/cancel', { path: { tripID: 5 } }]
	},
	{
		name: 'completeTrip',
		run: () => routes.completeTrip(5, body),
		verb: 'post',
		args: ['/trips/{tripID}/complete', { path: { tripID: 5 }, body }]
	},
	{
		name: 'listTripCrew',
		run: () => routes.listTripCrew(5),
		verb: 'get',
		args: ['/trips/{tripID}/crew', { path: { tripID: 5 } }]
	},
	{
		name: 'assignTripCrew',
		run: () => routes.assignTripCrew(5, body),
		verb: 'post',
		args: ['/trips/{tripID}/crew', { path: { tripID: 5 }, body }]
	},
	{
		name: 'removeTripCrew',
		run: () => routes.removeTripCrew(5, 9),
		verb: 'del',
		args: ['/trips/{tripID}/crew/{assignmentID}', { path: { tripID: 5, assignmentID: 9 } }]
	},
	{
		name: 'generateTripEnrollToken',
		run: () => routes.generateTripEnrollToken(5),
		verb: 'post',
		args: ['/trips/{tripID}/enroll-token', { path: { tripID: 5 } }]
	},
	{
		name: 'clearTripEnrollToken',
		run: () => routes.clearTripEnrollToken(5),
		verb: 'del',
		args: ['/trips/{tripID}/enroll-token', { path: { tripID: 5 } }]
	},
	{
		name: 'listTripEnrollments',
		run: () => routes.listTripEnrollments(5),
		verb: 'get',
		args: ['/trips/{tripID}/enrollments', { path: { tripID: 5 } }]
	},
	{
		name: 'updateTripEnrollmentStatus',
		run: () => routes.updateTripEnrollmentStatus(5, 9, 'accepted'),
		verb: 'put',
		args: [
			'/trips/{tripID}/enrollments/{id}/status',
			{ path: { tripID: 5, id: 9 }, body: { status: 'accepted' } }
		]
	},
	{
		name: 'deleteTripEnrollment',
		run: () => routes.deleteTripEnrollment(5, 9),
		verb: 'del',
		args: ['/trips/{tripID}/enrollments/{id}', { path: { tripID: 5, id: 9 } }]
	},

	{ name: 'listVoyages', run: () => routes.listVoyages(), verb: 'list', args: ['/voyages'] },
	{
		name: 'getVoyage',
		run: () => routes.getVoyage(5),
		verb: 'get',
		args: ['/voyages/{voyageID}', { path: { voyageID: 5 } }]
	},
	{
		name: 'createVoyage',
		run: () => routes.createVoyage(body),
		verb: 'post',
		args: ['/voyages', { body }]
	},
	{
		name: 'updateVoyage',
		run: () => routes.updateVoyage(5, body),
		verb: 'put',
		args: ['/voyages/{voyageID}', { path: { voyageID: 5 }, body }]
	},
	{
		name: 'deleteVoyage',
		run: () => routes.deleteVoyage(5),
		verb: 'del',
		args: ['/voyages/{voyageID}', { path: { voyageID: 5 } }]
	},
	{
		name: 'listVoyageCrew',
		run: () => routes.listVoyageCrew(5),
		verb: 'get',
		args: ['/voyages/{voyageID}/crew', { path: { voyageID: 5 } }]
	},
	{
		name: 'assignVoyageCrew',
		run: () => routes.assignVoyageCrew(5, body),
		verb: 'post',
		args: ['/voyages/{voyageID}/crew', { path: { voyageID: 5 }, body }]
	},
	{
		name: 'removeVoyageCrew',
		run: () => routes.removeVoyageCrew(5, 9),
		verb: 'del',
		args: ['/voyages/{voyageID}/crew/{assignmentID}', { path: { voyageID: 5, assignmentID: 9 } }]
	},
	{
		name: 'listVoyagePorts',
		run: () => routes.listVoyagePorts(5),
		verb: 'get',
		args: ['/voyages/{voyageID}/ports', { path: { voyageID: 5 } }]
	},
	{
		name: 'addVoyagePort',
		run: () => routes.addVoyagePort(5, body),
		verb: 'post',
		args: ['/voyages/{voyageID}/ports', { path: { voyageID: 5 }, body }]
	},
	{
		name: 'deleteVoyagePort',
		run: () => routes.deleteVoyagePort(5, 9),
		verb: 'del',
		args: ['/voyages/{voyageID}/ports/{portID}', { path: { voyageID: 5, portID: 9 } }]
	},
	{
		name: 'reorderVoyagePorts',
		run: () => routes.reorderVoyagePorts(5, [1, 2]),
		verb: 'put',
		args: ['/voyages/{voyageID}/ports/order', { path: { voyageID: 5 }, body: { port_ids: [1, 2] } }]
	},
	{
		name: 'listVoyageOpinions',
		run: () => routes.listVoyageOpinions(5),
		verb: 'get',
		args: ['/voyages/{voyageID}/opinions', { path: { voyageID: 5 } }]
	},
	{
		name: 'generateVoyageOpinion',
		run: () => routes.generateVoyageOpinion(5, body),
		verb: 'post',
		args: ['/voyages/{voyageID}/opinions', { path: { voyageID: 5 }, body }]
	},
	{
		name: 'deleteVoyageOpinion',
		run: () => routes.deleteVoyageOpinion(5, 9),
		verb: 'del',
		args: ['/voyages/{voyageID}/opinions/{opinionID}', { path: { voyageID: 5, opinionID: 9 } }]
	},
	{
		name: 'downloadVoyageOpinion',
		run: () => routes.downloadVoyageOpinion(5, 9),
		verb: 'download',
		args: [
			'/voyages/{voyageID}/opinions/{opinionID}/download',
			{ path: { voyageID: 5, opinionID: 9 } }
		]
	},

	{ name: 'listYachts', run: () => routes.listYachts(), verb: 'list', args: ['/yachts'] },
	{
		name: 'createYacht',
		run: () => routes.createYacht(body),
		verb: 'post',
		args: ['/yachts', { body }]
	},
	{
		name: 'deleteYacht',
		run: () => routes.deleteYacht(5),
		verb: 'del',
		args: ['/yachts/{yachtID}', { path: { yachtID: 5 } }]
	},

	{ name: 'listCrew', run: () => routes.listCrew(), verb: 'list', args: ['/crew'] },
	{
		name: 'listAssignableCrew',
		run: () => routes.listAssignableCrew(),
		verb: 'list',
		args: ['/crew']
	},
	{
		name: 'getCrew',
		run: () => routes.getCrew(5),
		verb: 'get',
		args: ['/crew/{crewID}', { path: { crewID: 5 } }]
	},
	{ name: 'createCrew', run: () => routes.createCrew(body), verb: 'post', args: ['/crew', { body }] },
	{
		name: 'deleteCrew',
		run: () => routes.deleteCrew(5),
		verb: 'del',
		args: ['/crew/{crewID}', { path: { crewID: 5 } }]
	},

	{ name: 'listTrainings', run: () => routes.listTrainings(), verb: 'list', args: ['/trainings'] },
	{
		name: 'createTraining',
		run: () => routes.createTraining(body),
		verb: 'post',
		args: ['/trainings', { body }]
	},
	{
		name: 'deleteTraining',
		run: () => routes.deleteTraining(5),
		verb: 'del',
		args: ['/trainings/{trainingID}', { path: { trainingID: 5 } }]
	},

	{ name: 'listCruises', run: () => routes.listCruises(), verb: 'list', args: ['/cruises'] },
	{
		name: 'getCruise',
		run: () => routes.getCruise(5),
		verb: 'get',
		args: ['/cruises/{cruiseID}', { path: { cruiseID: 5 } }]
	},
	{
		name: 'createCruise',
		run: () => routes.createCruise(body),
		verb: 'post',
		args: ['/cruises', { body }]
	},
	{
		name: 'updateCruise',
		run: () => routes.updateCruise(5, body),
		verb: 'put',
		args: ['/cruises/{cruiseID}', { path: { cruiseID: 5 }, body }]
	},
	{
		name: 'deleteCruise',
		run: () => routes.deleteCruise(5),
		verb: 'del',
		args: ['/cruises/{cruiseID}', { path: { cruiseID: 5 } }]
	},
	{
		name: 'listCruiseTrips',
		run: () => routes.listCruiseTrips(5),
		verb: 'get',
		args: ['/cruises/{cruiseID}/trips', { path: { cruiseID: 5 } }]
	},
	{
		name: 'listCruiseVoyages',
		run: () => routes.listCruiseVoyages(5),
		verb: 'get',
		args: ['/cruises/{cruiseID}/voyages', { path: { cruiseID: 5 } }]
	},
	{
		name: 'listCruiseEnrollments',
		run: () => routes.listCruiseEnrollments(5),
		verb: 'get',
		args: ['/cruises/{cruiseID}/enrollments', { path: { cruiseID: 5 } }]
	},
	{
		name: 'generateCruiseEnrollToken',
		run: () => routes.generateCruiseEnrollToken(5),
		verb: 'post',
		args: ['/cruises/{cruiseID}/enroll-token', { path: { cruiseID: 5 } }]
	},
	{
		name: 'clearCruiseEnrollToken',
		run: () => routes.clearCruiseEnrollToken(5),
		verb: 'del',
		args: ['/cruises/{cruiseID}/enroll-token', { path: { cruiseID: 5 } }]
	},
	{
		name: 'updateCruiseEnrollmentStatus',
		run: () => routes.updateCruiseEnrollmentStatus(5, 9, 'rejected'),
		verb: 'put',
		args: [
			'/cruises/{cruiseID}/enrollments/{enrollmentID}/status',
			{ path: { cruiseID: 5, enrollmentID: 9 }, body: { status: 'rejected' } }
		]
	},
	{
		name: 'assignCruiseEnrollmentTrip',
		run: () => routes.assignCruiseEnrollmentTrip(5, 9, 3),
		verb: 'put',
		args: [
			'/cruises/{cruiseID}/enrollments/{enrollmentID}/trip',
			{ path: { cruiseID: 5, enrollmentID: 9 }, body: { trip_id: 3 } }
		]
	},
	{
		name: 'deleteCruiseEnrollment',
		run: () => routes.deleteCruiseEnrollment(5, 9),
		verb: 'del',
		args: ['/cruises/{cruiseID}/enrollments/{enrollmentID}', { path: { cruiseID: 5, enrollmentID: 9 } }]
	},

	{ name: 'listMembers', run: () => routes.listMembers(), verb: 'get', args: ['/members'] },
	{
		name: 'updateMemberRole',
		run: () => routes.updateMemberRole(7, 'admin'),
		verb: 'put',
		args: ['/members/{userID}/role', { path: { userID: 7 }, body: { role: 'admin' } }]
	},

	{ name: 'getMe', run: () => routes.getMe(), verb: 'get', args: ['/auth/me'] },
	{ name: 'updateMe', run: () => routes.updateMe(body), verb: 'put', args: ['/auth/me', { body }] },
	{
		name: 'enroll',
		run: () => routes.enroll('tok', 'hello'),
		verb: 'post',
		args: ['/enroll/{token}', { path: { token: 'tok' }, body: { note: 'hello' } }]
	},
	{
		name: 'importXlsx',
		run: () => routes.importXlsx(fd),
		verb: 'upload',
		args: ['/import/xlsx', fd]
	},
	{
		name: 'importConfirm',
		run: () => routes.importConfirm(body),
		verb: 'post',
		args: ['/import/confirm', { body }]
	},
	{
		name: 'uploadImage',
		run: () => routes.uploadImage(fd),
		verb: 'upload',
		args: ['/upload/image', fd]
	}
];

describe('route helpers delegate to the api client', () => {
	it.each(cases)('$name', async ({ run, verb, args }) => {
		await run();
		expect(api[verb]).toHaveBeenCalledWith(...args);
	});
});

describe('geocode', () => {
	it('returns the matched places', async () => {
		const places = [{ name: 'Split', label: 'Split, HR', latitude: 1, longitude: 2 }];
		api.get.mockResolvedValueOnce(places);
		await expect(routes.geocode('Split')).resolves.toEqual(places);
		expect(api.get).toHaveBeenCalledWith('/geocode', { query: { q: 'Split' } });
	});

	it('falls back to an empty array when the proxy returns nothing', async () => {
		api.get.mockResolvedValueOnce(null);
		await expect(routes.geocode('zzz')).resolves.toEqual([]);
	});
});

describe('resolveEnroll', () => {
	it('builds the trip variant', async () => {
		api.get.mockResolvedValueOnce({ kind: 'trip', trip: { id: 1, name: 'Rejs' }, enrolled: false });
		const data = await routes.resolveEnroll('tok');
		expect(data.kind).toBe('trip');
		if (data.kind === 'trip') expect(data.trip.name).toBe('Rejs');
	});

	it('builds the cruise variant, defaulting trips to null', async () => {
		api.get.mockResolvedValueOnce({ kind: 'cruise', cruise: { id: 2, name: 'Event' } });
		const data = await routes.resolveEnroll('tok');
		expect(data.kind).toBe('cruise');
		if (data.kind === 'cruise') {
			expect(data.cruise.name).toBe('Event');
			expect(data.trips).toBeNull();
		}
	});

	it('throws when a cruise payload is missing its cruise', async () => {
		api.get.mockResolvedValueOnce({ kind: 'cruise' });
		await expect(routes.resolveEnroll('tok')).rejects.toThrow('Brak danych wydarzenia');
	});

	it('throws when a trip payload is missing its trip', async () => {
		api.get.mockResolvedValueOnce({ kind: 'trip' });
		await expect(routes.resolveEnroll('tok')).rejects.toThrow('Brak danych rejsu');
	});
});
