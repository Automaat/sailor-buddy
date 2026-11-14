import { expect, it } from 'vitest';
import { load } from './+page.js';

it('redirects the old members URL to the people directory', () => {
	expect(() => load({} as Parameters<typeof load>[0])).toThrow(
		expect.objectContaining({ status: 308, location: '/crew' })
	);
});
