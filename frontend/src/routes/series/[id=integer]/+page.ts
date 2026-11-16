import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

/** A series page moved to /rejsy/{id}; old links keep working. */
export const load: PageLoad = ({ params, url }) => {
	redirect(308, `/rejsy/${params.id}${url.search}`);
};
