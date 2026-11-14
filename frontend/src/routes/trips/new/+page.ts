import { listCruises } from '#lib/api/routes.js';
import { noteFailure, settle } from '#lib/api/errors.js';
import { cruiseIdParam } from '#lib/api/load.js';
import type { PageLoad } from './$types';

/**
 * Loads the event list for the planning form and the event a link preselects
 * with ?cruise_id=. A failed event list is named in the banner.
 */
export const load: PageLoad = async ({ url }) => {
	const loadErrors: string[] = [];
	const cruises = await settle(listCruises().then((r) => r ?? []), [], noteFailure(loadErrors, 'lista wydarzeń'));
	return { cruises, cruiseId: cruiseIdParam(url), loadErrors };
};
