import { getVoyage, listCruises, listYachts } from '#lib/api/routes.js';
import { noteFailure, settle } from '#lib/api/errors.js';
import { found } from '#lib/api/load.js';
import type { PageLoad } from './$types';

/**
 * Loads the voyage to edit with the yacht and event pickers' lists. The yacht
 * list is required: a picker without the saved yacht shows "-- Wybierz --",
 * so a save would clear it. A failed event list is named in the banner; the
 * event picker keeps the linked event selectable.
 */
export const load: PageLoad = async ({ params }) => {
	const id = Number(params.id);
	const loadErrors: string[] = [];
	const [voyage, yachts, cruises] = await Promise.all([
		found(getVoyage(id), {
			title: 'Nie znaleziono rejsu',
			message: 'Ten zrealizowany rejs nie istnieje lub nie masz do niego dostępu.',
			backHref: '/cruises?tab=completed',
			backLabel: 'Zrealizowane rejsy'
		}),
		listYachts().then((r) => r ?? []),
		settle(
			listCruises().then((r) => r ?? []),
			[],
			noteFailure(loadErrors, 'lista wydarzeń')
		)
	]);
	return { voyage, yachts, cruises, loadErrors };
};
