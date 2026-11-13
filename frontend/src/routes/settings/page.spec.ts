import { describe, it, expect } from 'vitest';
import { isRedirect } from '@sveltejs/kit';
import { load } from './+page.js';

describe('/settings', () => {
	it('redirects to the profile page', () => {
		let thrown: unknown;
		try {
			load();
		} catch (err) {
			thrown = err;
		}
		expect(isRedirect(thrown)).toBe(true);
		expect(thrown).toMatchObject({ status: 307, location: '/profile' });
	});
});
