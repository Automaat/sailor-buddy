import { getCruise, getTrip, getTripSignUp, listAssignableCrew, listTripCrew } from '#lib/api/routes.js';
import { noteFailure, settle } from '#lib/api/errors.js';
import { found } from '#lib/api/load.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }) => {
	const id = Number(params.id);
	const trip = await found(getTrip(id), {
		title: 'Nie znaleziono rejsu',
		message: 'Ten rejs nie istnieje lub nie masz do niego dostępu.',
		backHref: '/cruises',
		backLabel: 'Wszystkie rejsy'
	});
	const loadErrors: string[] = [];
	// Crew and the people list feed admin-only sections; the page shows their
	// failures only to an admin.
	const adminLoadErrors: string[] = [];
	const [crew, allCrewMembers, signUp, cruise] = await Promise.all([
		settle(listTripCrew(id).then((c) => c ?? []), [], noteFailure(adminLoadErrors, 'załoga')),
		settle(listAssignableCrew(), [], noteFailure(adminLoadErrors, 'lista osób')),
		settle(getTripSignUp(id), null, noteFailure(loadErrors, 'zapis na etap')),
		trip.cruise_id
			? settle(getCruise(trip.cruise_id), null, noteFailure(loadErrors, 'wydarzenie'))
			: Promise.resolve(null)
	]);
	return { trip, cruise, crew, allCrewMembers, signUp, loadErrors, adminLoadErrors };
};
