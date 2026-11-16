import { describe, it, expect, vi, beforeEach } from 'vitest';

const listCountriesMock = vi.fn();
vi.mock('#lib/api/routes.js', () => ({ listCountries: () => listCountriesMock() }));

import { countryName, formatCountries, loadCountries, searchCountries } from './countries.js';

const countries = [
	{ code: 'DK', name: 'Dania' },
	{ code: 'LT', name: 'Litwa' },
	{ code: 'LV', name: 'Łotwa' },
	{ code: 'MT', name: 'Malta' },
	{ code: 'NO', name: 'Norwegia' },
	{ code: 'SE', name: 'Szwecja' },
	{ code: 'TT', name: 'Trynidad i Tobago' }
];

describe('formatCountries', () => {
	it.each([
		[['NO'], 'Norwegia'],
		[['DK'], 'Dania'],
		[['SE', 'NO', 'DK'], 'Szwecja, Norwegia, Dania'],
		[[], ''],
		[null, ''],
		[undefined, ''],
		[['NO', 'QQ'], 'Norwegia, QQ']
	])('formats %j as %j', (codes, want) => {
		expect(formatCountries(codes, countries)).toBe(want);
	});

	it('shows codes while the list is missing', () => {
		expect(formatCountries(['NO', 'DK'], [])).toBe('NO, DK');
	});
});

describe('countryName', () => {
	it('names a listed code and falls back to the code', () => {
		expect(countryName('LV', countries)).toBe('Łotwa');
		expect(countryName('XX', countries)).toBe('XX');
	});
});

describe('searchCountries', () => {
	const names = (query: string) => searchCountries(countries, query).map((c) => c.name);

	it('lists every country in Polish order for a blank query', () => {
		expect(names('  ')).toEqual([
			'Dania',
			'Litwa',
			'Łotwa',
			'Malta',
			'Norwegia',
			'Szwecja',
			'Trynidad i Tobago'
		]);
	});

	it('ignores case and accents', () => {
		expect(names('lot')).toEqual(['Łotwa']);
		expect(names('ŁOT')).toEqual(['Łotwa']);
		expect(names('SZWE')).toEqual(['Szwecja']);
	});

	it('puts names that start with the query before names that contain it', () => {
		expect(names('wa')).toEqual(['Litwa', 'Łotwa']);
		expect(names('t')).toEqual(['Trynidad i Tobago', 'Litwa', 'Łotwa', 'Malta']);
	});

	it('finds a country by its code', () => {
		expect(names('no')).toEqual(['Norwegia']);
		expect(names('dk')).toEqual(['Dania']);
	});

	it('finds nothing for an unknown name', () => {
		expect(names('Atlantyda')).toEqual([]);
	});
});

describe('loadCountries', () => {
	beforeEach(() => listCountriesMock.mockReset());

	it('fetches the list once and retries after a failure', async () => {
		listCountriesMock.mockRejectedValueOnce(new Error('offline'));
		await expect(loadCountries()).rejects.toThrow('offline');

		listCountriesMock.mockResolvedValue(countries);
		await expect(loadCountries()).resolves.toBe(countries);
		await expect(loadCountries()).resolves.toBe(countries);
		expect(listCountriesMock).toHaveBeenCalledTimes(2);
	});
});
