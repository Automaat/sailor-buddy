import { describe, it, expect } from 'vitest';
import { safeRedirect } from './redirect.js';

describe('safeRedirect', () => {
	it.each([
		['/enroll/abc?x=1', '/enroll/abc?x=1'],
		[null, '/'],
		['', '/'],
		['https://evil.com', '/'],
		['//evil.com', '/'],
		['/\\evil.com', '/'],
		['cruises', '/']
	])('%j -> %j', (target, want) => {
		expect(safeRedirect(target)).toBe(want);
	});
});
