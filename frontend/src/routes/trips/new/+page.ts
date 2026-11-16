import { listCruises, listSeries } from '#lib/api/routes.js';
import { loadCountries } from '#lib/location/countries.js';
import { noteFailure, settle } from '#lib/api/errors.js';
import { cruiseIdParam } from '#lib/api/load.js';
import type { PageLoad } from './$types';

/**
 * Loads the series and event lists for the planning form's edition picker,
 * the country list and the edition a link preselects with ?cruise_id=. A
 * failed list is named in the banner.
 */
export const load: PageLoad = async ({ url }) => {
	const loadErrors: string[] = [];
	const [cruises, series, countries] = await Promise.all([
		settle(
			listCruises().then((r) => r ?? []),
			[],
			noteFailure(loadErrors, 'lista wydarzeń')
		),
		settle(listSeries(), [], noteFailure(loadErrors, 'lista serii')),
		settle(loadCountries(), [], noteFailure(loadErrors, 'lista krajów'))
	]);
	return { cruises, series, countries, cruiseId: cruiseIdParam(url), loadErrors };
};
