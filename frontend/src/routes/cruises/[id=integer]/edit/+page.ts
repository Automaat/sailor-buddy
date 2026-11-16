import { getCruise, listSeries } from '#lib/api/routes.js';
import { noteFailure, settle } from '#lib/api/errors.js';
import { found } from '#lib/api/load.js';
import { loadCountries } from '#lib/location/countries.js';
import type { PageLoad } from './$types';

/**
 * Loads the cruise to edit with the series picker's list. The series list is
 * required: a picker without the saved series shows "Bez serii", so a save
 * would unlink the cruise from it. A failed country list is named in the
 * banner; the picker keeps the saved countries.
 */
export const load: PageLoad = async ({ params }) => {
	const id = Number(params.id);
	const loadErrors: string[] = [];
	const [cruise, series, countries] = await Promise.all([
		found(getCruise(id), {
			title: 'Nie znaleziono wydarzenia',
			message: 'To wydarzenie nie istnieje lub nie masz do niego dostępu.',
			backHref: '/rejsy',
			backLabel: 'Wszystkie rejsy'
		}),
		listSeries().then((r) => r ?? []),
		settle(loadCountries(), [], noteFailure(loadErrors, 'lista krajów'))
	]);
	return { cruise, series, countries, loadErrors };
};
