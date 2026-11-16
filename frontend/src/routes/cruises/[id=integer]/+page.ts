import {
	getCruise,
	getSeries,
	listCruiseLegSignUps,
	listCruiseTrips,
	listCruiseVoyages
} from '#lib/api/routes.js';
import { noteFailure, settle } from '#lib/api/errors.js';
import { found } from '#lib/api/load.js';
import { loadCountries } from '#lib/location/countries.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }) => {
	const id = Number(params.id);
	const cruise = await found(getCruise(id), {
		title: 'Nie znaleziono wydarzenia',
		message: 'To wydarzenie nie istnieje lub nie masz do niego dostępu.',
		backHref: '/rejsy',
		backLabel: 'Wszystkie rejsy'
	});
	const loadErrors: string[] = [];
	const [trips, voyages, signUps, series, countries] = await Promise.all([
		settle(
			listCruiseTrips(id).then((r) => r ?? []),
			[],
			noteFailure(loadErrors, 'nadchodzące etapy')
		),
		settle(
			listCruiseVoyages(id).then((r) => r ?? []),
			[],
			noteFailure(loadErrors, 'zrealizowane etapy')
		),
		settle(
			listCruiseLegSignUps(id).then((r) => r ?? []),
			[],
			noteFailure(loadErrors, 'twoje zapisy na etapy')
		),
		cruise.series_id
			? settle(getSeries(cruise.series_id), null, noteFailure(loadErrors, 'cykl'))
			: Promise.resolve(null),
		settle(loadCountries(), [], noteFailure(loadErrors, 'lista krajów'))
	]);
	return { cruise, trips, voyages, signUps, series, countries, loadErrors };
};
