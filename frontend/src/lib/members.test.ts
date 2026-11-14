import { describe, it, expect } from 'vitest';
import { makeMember } from '#lib/test-utils.js';
import { filterCounts, filterMembers, fold, type MemberFilter } from './members.js';

const roster = [
	makeMember({
		id: 1,
		name: 'Łukasz Żółć',
		email: 'lukasz@agh.dev',
		membership: 'czlonek',
		is_captain: true
	}),
	makeMember({ id: 2, name: 'Anna Nowak', email: 'anna@x.dev', membership: 'sympatyk' }),
	makeMember({
		id: 3,
		name: 'Gość Bez Profilu',
		email: 'guest@x.dev',
		membership: 'gosc',
		onboarded: false
	})
];

const ids = (list: { id: number }[]) => list.map((m) => m.id);

describe('fold', () => {
	it('lowercases and strips Polish diacritics', () => {
		expect(fold('Łukasz ŻÓŁĆ Gość')).toBe('lukasz zolc gosc');
	});
});

describe('filterMembers', () => {
	it.each<[string, MemberFilter, string, number[]]>([
		['no query, all', 'all', '', [1, 2, 3]],
		['blank query', 'all', '   ', [1, 2, 3]],
		['name, case and diacritics insensitive', 'all', 'zolc', [1]],
		['e-mail fragment', 'all', 'X.DEV', [2, 3]],
		['no match', 'all', 'zzz', []],
		['membership chip', 'sympatyk', '', [2]],
		['captain chip', 'captain', '', [1]],
		['incomplete profile chip', 'incomplete', '', [3]],
		['chip and query combine', 'gosc', 'anna', []]
	])('%s', (_name, filter, query, want) => {
		expect(ids(filterMembers(roster, query, filter))).toEqual(want);
	});
});

describe('filterCounts', () => {
	it('counts every chip over the whole list', () => {
		expect(filterCounts(roster)).toEqual({
			all: 3,
			czlonek: 1,
			sympatyk: 1,
			gosc: 1,
			captain: 1,
			incomplete: 1
		});
	});
});
