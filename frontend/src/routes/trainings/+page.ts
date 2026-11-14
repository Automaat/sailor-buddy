import { listTrainings } from '#lib/api/routes.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async () => {
	return { trainings: (await listTrainings()) ?? [] };
};
