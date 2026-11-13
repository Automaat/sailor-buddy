import { describe, it, expect } from 'vitest';
import { plural } from './plural.js';

describe('plural', () => {
	it.each([
		[0, 'rejsów'],
		[1, 'rejs'],
		[2, 'rejsy'],
		[3, 'rejsy'],
		[4, 'rejsy'],
		[5, 'rejsów'],
		[11, 'rejsów'],
		[12, 'rejsów'],
		[13, 'rejsów'],
		[14, 'rejsów'],
		[21, 'rejsów'],
		[22, 'rejsy'],
		[24, 'rejsy'],
		[25, 'rejsów'],
		[101, 'rejsów'],
		[112, 'rejsów'],
		[122, 'rejsy']
	])('picks the form for %i', (n, want) => {
		expect(plural(n, 'rejs', 'rejsy', 'rejsów')).toBe(want);
	});
});
