import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

/** The old Rejsy list became the series list; its links keep working. */
export const load: PageLoad = () => {
	redirect(308, '/rejsy');
};
