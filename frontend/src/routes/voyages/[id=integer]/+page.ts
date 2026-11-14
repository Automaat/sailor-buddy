import {
	getCruise,
	getVoyage,
	listAssignableCrew,
	listVoyageCrew,
	listVoyageOpinions,
	listVoyagePorts,
	listYachts
} from '#lib/api/routes.js';
import { noteFailure, settle } from '#lib/api/errors.js';
import { found } from '#lib/api/load.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }) => {
	const id = Number(params.id);
	const voyage = await found(getVoyage(id), {
		title: 'Nie znaleziono rejsu',
		message: 'Ten zrealizowany rejs nie istnieje lub nie masz do niego dostępu.',
		backHref: '/cruises?tab=completed',
		backLabel: 'Zrealizowane rejsy'
	});
	const loadErrors: string[] = [];
	// Crew, opinions and the people list feed admin-only sections; the page
	// shows their failures only to an admin.
	const adminLoadErrors: string[] = [];
	const [crew, opinions, ports, allCrewMembers, yachts, cruise] = await Promise.all([
		settle(listVoyageCrew(id).then((c) => c ?? []), [], noteFailure(adminLoadErrors, 'załoga')),
		settle(listVoyageOpinions(id).then((o) => o ?? []), [], noteFailure(adminLoadErrors, 'opinie')),
		settle(listVoyagePorts(id).then((p) => p ?? []), [], noteFailure(loadErrors, 'porty')),
		settle(listAssignableCrew(), [], noteFailure(adminLoadErrors, 'lista osób')),
		settle(listYachts().then((y) => y ?? []), [], noteFailure(loadErrors, 'jachty')),
		voyage.cruise_id
			? settle(getCruise(voyage.cruise_id), null, noteFailure(loadErrors, 'wydarzenie'))
			: Promise.resolve(null)
	]);
	return { voyage, cruise, crew, opinions, ports, allCrewMembers, yachts, loadErrors, adminLoadErrors };
};
