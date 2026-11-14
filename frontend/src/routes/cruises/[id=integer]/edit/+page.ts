import { getCruise, listSeries } from '#lib/api/routes.js';
import { found } from '#lib/api/load.js';
import type { PageLoad } from './$types';

/**
 * Loads the cruise to edit with the series picker's list. The series list is
 * required: a picker without the saved series shows "Bez serii", so a save
 * would unlink the cruise from it.
 */
export const load: PageLoad = async ({ params }) => {
	const id = Number(params.id);
	const [cruise, series] = await Promise.all([
		found(getCruise(id), {
			title: 'Nie znaleziono wydarzenia',
			message: 'To wydarzenie nie istnieje lub nie masz do niego dostępu.',
			backHref: '/cruises',
			backLabel: 'Wszystkie rejsy'
		}),
		listSeries().then((r) => r ?? [])
	]);
	return { cruise, series };
};
