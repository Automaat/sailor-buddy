import { getCruise, getTrip, getTripSignUp, listTripCrew } from '#lib/api/routes.js';
import { noteFailure, settle } from '#lib/api/errors.js';
import { found } from '#lib/api/load.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }) => {
	const id = Number(params.id);
	const trip = await found(getTrip(id), {
		title: 'Nie znaleziono rejsu',
		message: 'Ten rejs nie istnieje lub nie masz do niego dostępu.',
		backHref: '/series',
		backLabel: 'Wszystkie rejsy'
	});
	const loadErrors: string[] = [];
	// The crew feeds a section only admins and the leg's captain see; the
	// page shows its failure only to them.
	const crewLoadErrors: string[] = [];
	const [crew, signUp, cruise] = await Promise.all([
		settle(
			listTripCrew(id).then((c) => c ?? []),
			[],
			noteFailure(crewLoadErrors, 'załoga')
		),
		settle(getTripSignUp(id), null, noteFailure(loadErrors, 'zapis na etap')),
		trip.cruise_id
			? settle(getCruise(trip.cruise_id), null, noteFailure(loadErrors, 'wydarzenie'))
			: Promise.resolve(null)
	]);
	return { trip, cruise, crew, signUp, loadErrors, crewLoadErrors };
};
