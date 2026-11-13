import { describe, it, expect } from 'vitest';
import { makeCruise, makeTrip, makeVoyage } from '#lib/test-utils.js';
import { buildRejsEntries, filterRejsEntries, parseRejsTab } from './rejsy.js';

describe('buildRejsEntries', () => {
	it('lists each cruise once with its legs summarized', () => {
		const entries = buildRejsEntries(
			[makeCruise({ id: 7, name: 'NNE 2025', enroll_token: 'tok' })],
			[makeTrip({ id: 1, cruise_id: 7 }), makeTrip({ id: 2, cruise_id: 7 })],
			[makeVoyage({ id: 3, cruise_id: 7 })]
		);
		expect(entries).toHaveLength(1);
		expect(entries[0]).toMatchObject({
			key: 'cruise-7',
			href: '/cruises/7',
			plannedLegs: 2,
			completedLegs: 1,
			status: 'upcoming',
			enrollOpen: true
		});
	});

	it('ignores legs of other cruises', () => {
		const [entry] = buildRejsEntries(
			[makeCruise({ id: 1 })],
			[makeTrip({ cruise_id: 2 })],
			[makeVoyage({ cruise_id: 2, miles: 99 })]
		);
		expect(entry).toMatchObject({ plannedLegs: 0, completedLegs: 0, miles: 0 });
	});

	it.each([
		{ name: 'all legs sailed', trips: [], voyages: [makeVoyage({ cruise_id: 1 })], want: 'completed' },
		{ name: 'a leg still planned', trips: [makeTrip({ cruise_id: 1 })], voyages: [makeVoyage({ cruise_id: 1 })], want: 'upcoming' },
		{ name: 'only cancelled legs', trips: [makeTrip({ cruise_id: 1, status: 'cancelled' })], voyages: [], want: 'cancelled' },
		{ name: 'cancelled leg plus a sailed one', trips: [makeTrip({ cruise_id: 1, status: 'cancelled' })], voyages: [makeVoyage({ cruise_id: 1 })], want: 'completed' },
		{ name: 'no legs yet', trips: [], voyages: [], want: 'upcoming' }
	])('marks a cruise with $name as $want', ({ trips, voyages, want }) => {
		const [entry] = buildRejsEntries([makeCruise({ id: 1 })], trips, voyages);
		expect(entry.status).toBe(want);
	});

	it('sums hours and miles over the sailed legs', () => {
		const [entry] = buildRejsEntries(
			[makeCruise({ id: 1 })],
			[],
			[makeVoyage({ cruise_id: 1, miles: 100, hours_total: 30 }), makeVoyage({ id: 2, cruise_id: 1, miles: 50, hours_total: 12 })]
		);
		expect(entry).toMatchObject({ miles: 150, hoursTotal: 42 });
	});

	it('falls back to the earliest leg date when the cruise has none', () => {
		const [entry] = buildRejsEntries(
			[makeCruise({ id: 1, embark_date: undefined })],
			[makeTrip({ cruise_id: 1, embark_date: '2026-08-10' })],
			[makeVoyage({ cruise_id: 1, embark_date: '2026-07-01' })]
		);
		expect(entry.embarkDate).toBe('2026-07-01');
	});

	it('prefers the cruise date over leg dates', () => {
		const [entry] = buildRejsEntries(
			[makeCruise({ id: 1, embark_date: '2026-09-01' })],
			[makeTrip({ cruise_id: 1, embark_date: '2026-07-01' })],
			[]
		);
		expect(entry.embarkDate).toBe('2026-09-01');
	});
});

describe('filterRejsEntries', () => {
	const entries = buildRejsEntries(
		[
			makeCruise({ id: 1, name: 'Late', embark_date: '2026-09-01' }),
			makeCruise({ id: 2, name: 'Soon', embark_date: '2026-05-01' }),
			makeCruise({ id: 3, name: 'Undated', embark_date: undefined }),
			makeCruise({ id: 4, name: 'Cancelled', embark_date: '2026-06-01' }),
			makeCruise({ id: 5, name: 'Old', embark_date: '2020-07-01' }),
			makeCruise({ id: 6, name: 'Recent', embark_date: '2025-07-01' })
		],
		[makeTrip({ id: 40, cruise_id: 4, status: 'cancelled', embark_date: undefined })],
		[
			makeVoyage({ id: 50, cruise_id: 5, embark_date: undefined }),
			makeVoyage({ id: 60, cruise_id: 6, embark_date: undefined })
		]
	);
	const names = (tab: 'upcoming' | 'completed' | 'all') => filterRejsEntries(entries, tab).map((e) => e.name);

	it('shows upcoming soonest first with undated last', () => {
		expect(names('upcoming')).toEqual(['Soon', 'Late', 'Undated']);
	});

	it('shows completed newest first', () => {
		expect(names('completed')).toEqual(['Recent', 'Old']);
	});

	it('shows everything, cancelled included, newest first', () => {
		expect(names('all')).toEqual(['Late', 'Cancelled', 'Soon', 'Recent', 'Old', 'Undated']);
	});
});

describe('parseRejsTab', () => {
	it.each([
		['completed', 'completed'],
		['all', 'all'],
		['upcoming', 'upcoming'],
		[null, 'upcoming'],
		['bogus', 'upcoming']
	])('maps %s to %s', (input, want) => {
		expect(parseRejsTab(input)).toBe(want);
	});
});
