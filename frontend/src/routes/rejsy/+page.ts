import { listSeries, listStandaloneLegs } from '#lib/api/routes.js';
import { noteFailure, settle } from '#lib/api/errors.js';
import type { PageLoad } from './$types';

/**
 * Loads the Rejsy page: the club's series and the "Bez serii" entries. A
 * failed section is named in the banner instead of shown as an empty list.
 */
export const load: PageLoad = async () => {
	const loadErrors: string[] = [];
	const [series, standalone] = await Promise.all([
		settle(listSeries(), [], noteFailure(loadErrors, 'serie')),
		settle(listStandaloneLegs(), [], noteFailure(loadErrors, 'rejsy bez serii'))
	]);
	return { series, standalone, loadErrors };
};
