import { getDashboard, listMyLegs } from '#lib/api/routes.js';
import { noteFailure, settle } from '#lib/api/errors.js';
import type { PageLoad } from './$types';

/**
 * Loads the caller's upcoming legs and sea time. A failed section is null,
 * so the page names it in the banner instead of showing an empty list.
 */
export const load: PageLoad = async () => {
	const loadErrors: string[] = [];
	const [legs, stats] = await Promise.all([
		settle(listMyLegs(), null, noteFailure(loadErrors, 'Twoje etapy')),
		settle(getDashboard(), null, noteFailure(loadErrors, 'statystyki'))
	]);
	return { legs, stats, loadErrors };
};
