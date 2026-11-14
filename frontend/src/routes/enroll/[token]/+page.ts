import { resolveEnroll } from '#lib/api/routes.js';
import { found } from '#lib/api/load.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }) => {
	const enroll = await found(resolveEnroll(params.token), {
		title: 'Nieprawidłowy link',
		message: 'Ten link zapisu jest nieprawidłowy lub został wyłączony.',
		backHref: '/',
		backLabel: 'Wróć na start'
	});
	return { token: params.token, enroll };
};
