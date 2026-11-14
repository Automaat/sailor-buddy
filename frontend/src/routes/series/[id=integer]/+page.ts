import { getSeries, listSeriesEditions } from '#lib/api/routes.js';
import { found } from '#lib/api/load.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }) => {
	const id = Number(params.id);
	const [series, editions] = await Promise.all([
		found(getSeries(id), {
			title: 'Nie znaleziono serii',
			message: 'Ta seria nie istnieje lub nie masz do niej dostępu.',
			backHref: '/series',
			backLabel: 'Wszystkie serie'
		}),
		listSeriesEditions(id).then((r) => r ?? [])
	]);
	return { series, editions };
};
