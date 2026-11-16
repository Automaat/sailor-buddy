import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

/**
 * Rendered on the server, unlike the rest of the app (`ssr = false` in the
 * root layout), so a bookmarked or shared link gets an HTTP 308.
 */
export const ssr = true;

/** A series page moved to /rejsy/{id}; old links keep working. */
export const load: PageLoad = ({ params, url }) => {
	redirect(308, `/rejsy/${params.id}${url.search}`);
};
