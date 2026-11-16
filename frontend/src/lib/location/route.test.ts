import { describe, it, expect } from 'vitest';
import { formatRoute, MISSING_PORT } from './route.js';

describe('formatRoute', () => {
	it.each([
		['both ports', { name: 'Gdynia' }, { name: 'Visby' }, 'Gdynia → Visby'],
		['start only', { name: 'Gdynia' }, undefined, `Gdynia → ${MISSING_PORT}`],
		['end only', null, { name: 'Visby' }, `${MISSING_PORT} → Visby`],
		['neither', undefined, null, null],
		[
			'ports with coordinates',
			{ name: 'Hel', latitude: 54.6, longitude: 18.8 },
			{ name: 'Hel' },
			'Hel → Hel'
		]
	])('%s', (_, start, end, want) => {
		expect(formatRoute(start, end)).toBe(want);
	});
});
