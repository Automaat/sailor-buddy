import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

/** The Rejsy page moved to /rejsy; old links keep working. */
export const load: PageLoad = ({ url }) => {
	redirect(308, `/rejsy${url.search}`);
};
