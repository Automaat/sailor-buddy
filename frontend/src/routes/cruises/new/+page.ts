import { getSeries } from '#lib/api/routes.js';
import { noteFailure, settle } from '#lib/api/errors.js';
import { found, seriesIdParam, type NotFoundCopy } from '#lib/api/load.js';
import { loadCountries } from '#lib/location/countries.js';
import type { PageLoad } from './$types';

const notFoundCopy: NotFoundCopy = {
	title: 'Nie znaleziono serii',
	message: 'Nowa edycja powstaje na stronie swojej serii. Ta seria nie istnieje.',
	backHref: '/rejsy',
	backLabel: 'Wszystkie rejsy'
};

/**
 * Loads the series a new edition belongs to (?series_id=), without which the
 * page is not found, and the country list, whose failure the banner names.
 */
export const load: PageLoad = async ({ url }) => {
	const id = seriesIdParam(url);
	const loadErrors: string[] = [];
	const [series, countries] = await Promise.all([
		found(id ? getSeries(id) : Promise.resolve(null), notFoundCopy),
		settle(loadCountries(), [], noteFailure(loadErrors, 'lista krajów'))
	]);
	return { series, countries, loadErrors };
};
