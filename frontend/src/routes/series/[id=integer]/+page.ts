import { getSeries, listSeriesEditions } from '#lib/api/routes.js';
import { found, type NotFoundCopy } from '#lib/api/load.js';
import type { PageLoad } from './$types';

const notFoundCopy: NotFoundCopy = {
	title: 'Nie znaleziono serii',
	message: 'Ta seria nie istnieje lub nie masz do niej dostępu.',
	backHref: '/series',
	backLabel: 'Wszystkie serie'
};

export const load: PageLoad = async ({ params }) => {
	const id = Number(params.id);
	const [series, editions] = await Promise.all([
		found(getSeries(id), notFoundCopy),
		found(
			listSeriesEditions(id).then((r) => r ?? []),
			notFoundCopy
		)
	]);
	return { series, editions };
};
