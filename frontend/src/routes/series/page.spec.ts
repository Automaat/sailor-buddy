import { describe, it, expect } from 'vitest';
import { isRedirect } from '@sveltejs/kit';
import { load } from './+page.js';

describe('old Rejsy path', () => {
	it.each([
		['/series', '/rejsy'],
		['/series?x=1', '/rejsy?x=1'],
		['/series?tab=all&mine=1', '/rejsy?tab=all&mine=1']
	])('redirects %s to %s', (path, location) => {
		let thrown: unknown;
		try {
			void load({ url: new URL(`http://localhost${path}`) } as Parameters<typeof load>[0]);
		} catch (e) {
			thrown = e;
		}
		expect(isRedirect(thrown)).toBe(true);
		expect(thrown).toMatchObject({ status: 308, location });
	});
});
