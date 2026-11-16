import { getTrip, listCruises, listYachts } from '#lib/api/routes.js';
import { noteFailure, settle } from '#lib/api/errors.js';
import { found } from '#lib/api/load.js';
import { loadCountries } from '#lib/location/countries.js';
import type { PageLoad } from './$types';

/**
 * Loads the trip to edit with the yacht and event pickers' lists. The yacht
 * list is required: a picker without the saved yacht shows "-- Wybierz --",
 * so a save would clear it. A failed event or country list is named in the
 * banner; the pickers keep the saved event and countries.
 */
export const load: PageLoad = async ({ params }) => {
	const id = Number(params.id);
	const loadErrors: string[] = [];
	const [trip, yachts, cruises, countries] = await Promise.all([
		found(getTrip(id), {
			title: 'Nie znaleziono rejsu',
			message: 'Ten rejs nie istnieje lub nie masz do niego dostępu.',
			backHref: '/rejsy',
			backLabel: 'Wszystkie rejsy'
		}),
		listYachts().then((r) => r ?? []),
		settle(
			listCruises().then((r) => r ?? []),
			[],
			noteFailure(loadErrors, 'lista wydarzeń')
		),
		settle(loadCountries(), [], noteFailure(loadErrors, 'lista krajów'))
	]);
	return { trip, yachts, cruises, countries, loadErrors };
};
