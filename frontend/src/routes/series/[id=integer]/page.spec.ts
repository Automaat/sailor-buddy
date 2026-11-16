import { describe, it, expect } from 'vitest';
import { isRedirect } from '@sveltejs/kit';
import { load } from './+page.js';

describe('old series path', () => {
	it.each([
		['/series/5', '/rejsy/5', '5'],
		['/series/5?x=1', '/rejsy/5?x=1', '5'],
		['/series/12?a=1&b=2', '/rejsy/12?a=1&b=2', '12']
	])('redirects %s to %s', (path, location, id) => {
		let thrown: unknown;
		try {
			void load({
				params: { id },
				url: new URL(`http://localhost${path}`)
			} as Parameters<typeof load>[0]);
		} catch (e) {
			thrown = e;
		}
		expect(isRedirect(thrown)).toBe(true);
		expect(thrown).toMatchObject({ status: 308, location });
	});
});
