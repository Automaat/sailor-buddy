import { describe, it, expect } from 'vitest';
import { isRedirect } from '@sveltejs/kit';
import { load } from './+page.js';

describe('old Rejsy list', () => {
	it.each(['/cruises', '/cruises?tab=all', '/cruises?tab=completed&mine=1'])(
		'redirects %s to the Rejsy page',
		(path) => {
			let thrown: unknown;
			try {
				void load({ url: new URL(`http://localhost${path}`) } as Parameters<typeof load>[0]);
			} catch (e) {
				thrown = e;
			}
			expect(isRedirect(thrown)).toBe(true);
			expect(thrown).toMatchObject({ status: 308, location: '/series' });
		}
	);
});
