import { describe, it, expect } from 'vitest';
import { makePerson } from '#lib/test-utils.js';
import { matchesAccount, matchesPerson, parseOtherNames } from './people.js';

describe('people helpers', () => {
	const anna = makePerson({
		full_name: 'Anna Nowak',
		other_names: ['Anna Kowalska'],
		email: 'anna@example.dev',
		has_account: true
	});

	it.each([
		['', true],
		['  ', true],
		['nowak', true],
		['KOWALSKA', true],
		['anna@ex', true],
		['wisniewska', false]
	])('matchesPerson(%j) = %s', (query, want) => {
		expect(matchesPerson(anna, query)).toBe(want);
	});

	it('tolerates a missing other-names list', () => {
		expect(matchesPerson(makePerson({ full_name: 'Jan', other_names: null }), 'jan')).toBe(true);
	});

	it('filters by account', () => {
		expect(matchesAccount(anna, 'with')).toBe(true);
		expect(matchesAccount(anna, 'without')).toBe(false);
		expect(matchesAccount(anna, 'all')).toBe(true);
	});

	it('splits other names on commas and newlines', () => {
		expect(parseOtherNames(' Ewa Kos,\nEwa Majówna, ,')).toEqual(['Ewa Kos', 'Ewa Majówna']);
		expect(parseOtherNames('')).toEqual([]);
	});
});
