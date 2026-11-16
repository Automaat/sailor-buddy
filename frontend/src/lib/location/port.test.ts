import { describe, it, expect } from 'vitest';
import { hasCoordinates, portBody, portDraft, samePortName } from './port.js';

describe('portBody', () => {
	it.each([
		['empty name is omitted', { name: '' }, undefined],
		['blank name is omitted', { name: '  \t ' }, undefined],
		['blank name drops its coordinates', { name: ' ', latitude: 54.6, longitude: 18.8 }, undefined],
		['name only', { name: '  Nowa   Wieś ' }, { name: 'Nowa Wieś' }],
		[
			'name and coordinates',
			{ name: 'Hel', latitude: 54.6, longitude: 18.8 },
			{ name: 'Hel', latitude: 54.6, longitude: 18.8 }
		],
		['half a pair is dropped', { name: 'Hel', latitude: 54.6 }, { name: 'Hel' }]
	])('%s', (_, draft, want) => {
		expect(portBody(draft)).toEqual(want);
	});
});

describe('portDraft', () => {
	it.each([
		['no port', undefined, { name: '' }],
		['null port', null, { name: '' }],
		['name only', { name: 'Visby' }, { name: 'Visby' }],
		[
			'located port',
			{ name: 'Hel', latitude: 54.6, longitude: 18.8 },
			{ name: 'Hel', latitude: 54.6, longitude: 18.8 }
		]
	])('%s', (_, port, want) => {
		expect(portDraft(port)).toEqual(want);
	});
});

describe('samePortName', () => {
	it.each([
		['Hel', ' Hel ', true],
		['Nowa Wieś', 'Nowa   Wieś', true],
		['Hel', 'Hela', false],
		['Hel', 'hel', false]
	])('%s vs %s', (a, b, want) => {
		expect(samePortName(a, b)).toBe(want);
	});
});

describe('hasCoordinates', () => {
	it.each([
		[{ latitude: 0, longitude: 0 }, true],
		[{ latitude: 54.6 }, false],
		[{ latitude: null, longitude: null }, false],
		[undefined, false]
	])('%j', (p, want) => {
		expect(hasCoordinates(p)).toBe(want);
	});
});
