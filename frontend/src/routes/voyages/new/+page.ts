import { listCruises, listYachts } from '#lib/api/routes.js';
import { noteFailure, settle } from '#lib/api/errors.js';
import { cruiseIdParam } from '#lib/api/load.js';
import type { PageLoad } from './$types';

/**
 * Loads the yacht and event lists for the voyage form and the event a link
 * preselects with ?cruise_id=. A failed list is named in the banner.
 */
export const load: PageLoad = async ({ url }) => {
	const loadErrors: string[] = [];
	const [yachts, cruises] = await Promise.all([
		settle(
			listYachts().then((r) => r ?? []),
			[],
			noteFailure(loadErrors, 'jachty')
		),
		settle(
			listCruises().then((r) => r ?? []),
			[],
			noteFailure(loadErrors, 'lista wydarzeń')
		)
	]);
	return { yachts, cruises, cruiseId: cruiseIdParam(url), loadErrors };
};
