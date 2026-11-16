import { getSeries } from '#lib/api/routes.js';
import { found, seriesIdParam, type NotFoundCopy } from '#lib/api/load.js';
import type { PageLoad } from './$types';

const notFoundCopy: NotFoundCopy = {
	title: 'Nie znaleziono serii',
	message: 'Nowa edycja powstaje na stronie swojej serii. Ta seria nie istnieje.',
	backHref: '/rejsy',
	backLabel: 'Wszystkie rejsy'
};

/** Loads the series a new edition belongs to (?series_id=); without one the page is not found. */
export const load: PageLoad = async ({ url }) => {
	const id = seriesIdParam(url);
	const series = await found(id ? getSeries(id) : Promise.resolve(null), notFoundCopy);
	return { series };
};
