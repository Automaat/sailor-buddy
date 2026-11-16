import { resolveEnroll } from '#lib/api/routes.js';
import { noteFailure, settle } from '#lib/api/errors.js';
import { found } from '#lib/api/load.js';
import { loadCountries } from '#lib/location/countries.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }) => {
	const loadErrors: string[] = [];
	const [enroll, countries] = await Promise.all([
		found(resolveEnroll(params.token), {
			title: 'Nieprawidłowy link',
			message: 'Ten link zapisu jest nieprawidłowy lub został wyłączony.',
			backHref: '/',
			backLabel: 'Wróć na start'
		}),
		settle(loadCountries(), [], noteFailure(loadErrors, 'lista krajów'))
	]);
	return { token: params.token, enroll, countries, loadErrors };
};
