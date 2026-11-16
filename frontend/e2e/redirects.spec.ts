import { expect, test } from '@playwright/test';

// The old Rejsy URLs must answer with a real HTTP redirect, not only a
// client-side one, so bookmarks, shared links and crawlers follow them.
const cases = [
	['/series', '/rejsy'],
	['/series?x=1', '/rejsy?x=1'],
	['/series/5', '/rejsy/5'],
	['/series/5?x=1&y=2', '/rejsy/5?x=1&y=2'],
	['/cruises', '/rejsy'],
	['/cruises?tab=all', '/rejsy']
] as const;

for (const [from, to] of cases) {
	test(`${from} answers 308 to ${to}`, async ({ request }) => {
		const response = await request.get(from, { maxRedirects: 0 });
		expect(response.status()).toBe(308);
		expect(response.headers()['location']).toBe(to);
	});
}
