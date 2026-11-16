import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

/**
 * Rendered on the server, unlike the rest of the app (`ssr = false` in the
 * root layout), so a bookmarked or shared link gets an HTTP 308.
 */
export const ssr = true;

/** The old Rejsy list became the series list; its links keep working. */
export const load: PageLoad = () => {
	redirect(308, '/rejsy');
};
