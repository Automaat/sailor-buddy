import {
	getCruise,
	getVoyage,
	listAssignableCrew,
	listVoyageCrew,
	listVoyageOpinions,
	listVoyagePorts
} from '#lib/api/routes.js';
import { noteFailure, settle } from '#lib/api/errors.js';
import { found } from '#lib/api/load.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }) => {
	const id = Number(params.id);
	const voyage = await found(getVoyage(id), {
		title: 'Nie znaleziono rejsu',
		message: 'Ten zrealizowany rejs nie istnieje lub nie masz do niego dostępu.',
		backHref: '/series',
		backLabel: 'Wszystkie rejsy'
	});
	const loadErrors: string[] = [];
	// Crew and opinions feed sections only admins and the voyage captain see,
	// the people list an admin-only form; the page shows each failure only to
	// those who see its section.
	const crewLoadErrors: string[] = [];
	const adminLoadErrors: string[] = [];
	const [crew, opinions, ports, allCrewMembers, cruise] = await Promise.all([
		settle(
			listVoyageCrew(id).then((c) => c ?? []),
			[],
			noteFailure(crewLoadErrors, 'załoga')
		),
		settle(
			listVoyageOpinions(id).then((o) => o ?? []),
			[],
			noteFailure(crewLoadErrors, 'opinie')
		),
		settle(
			listVoyagePorts(id).then((p) => p ?? []),
			[],
			noteFailure(loadErrors, 'porty')
		),
		settle(listAssignableCrew(), [], noteFailure(adminLoadErrors, 'lista osób')),
		voyage.cruise_id
			? settle(getCruise(voyage.cruise_id), null, noteFailure(loadErrors, 'wydarzenie'))
			: Promise.resolve(null)
	]);
	return {
		voyage,
		cruise,
		crew,
		opinions,
		ports,
		allCrewMembers,
		loadErrors,
		crewLoadErrors,
		adminLoadErrors
	};
};
