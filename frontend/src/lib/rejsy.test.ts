import { describe, it, expect, beforeAll, afterAll, vi } from 'vitest';
import { makeCruise, makeTrip, makeVoyage } from '#lib/test-utils.js';
import {
	buildRejsEntries,
	filterRejsEntries,
	isLegOverdue,
	isMineFilter,
	legStatus,
	onlyMine,
	parseRejsTab,
	rejsListUrl,
	rejsStatusLabels,
	warsawToday
} from './rejsy.js';

// The factories' default leg (1-7 Jun 2025) is still ahead on this clock.
beforeAll(() => {
	vi.useFakeTimers({ toFake: ['Date'] });
	vi.setSystemTime(new Date('2025-05-01T12:00:00Z'));
});
afterAll(() => {
	vi.useRealTimers();
});

describe('warsawToday', () => {
	it.each([
		// 23:30 UTC on 9 Aug is 01:30 on 10 Aug in Warsaw (CEST, UTC+2).
		['2026-08-09T23:30:00Z', '2026-08-10'],
		// 23:30 UTC on 14 Jan is 00:30 on 15 Jan in Warsaw (CET, UTC+1).
		['2026-01-14T23:30:00Z', '2026-01-15'],
		['2026-01-14T22:30:00Z', '2026-01-14']
	])('%s -> %s', (instant, want) => {
		expect(warsawToday(new Date(instant))).toBe(want);
	});

	it('reads the current clock by default', () => {
		expect(warsawToday()).toBe('2025-05-01');
	});
});

describe('isLegOverdue', () => {
	const today = '2026-08-10';
	it.each([
		{ name: 'ended yesterday', leg: { status: 'planned', disembark_date: '2026-08-09' }, cruise: {}, want: true },
		{ name: 'ends today', leg: { status: 'planned', disembark_date: '2026-08-10' }, cruise: {}, want: false },
		{ name: 'ends tomorrow', leg: { status: 'planned', disembark_date: '2026-08-11' }, cruise: {}, want: false },
		{ name: 'no dates at all', leg: { status: 'planned' }, cruise: {}, want: false },
		{ name: 'no leg date, cruise ended', leg: { status: 'planned' }, cruise: { disembark_date: '2026-08-09' }, want: true },
		{ name: 'no leg date, cruise ends today', leg: { status: 'planned' }, cruise: { disembark_date: '2026-08-10' }, want: false },
		{ name: 'leg date wins over cruise date', leg: { status: 'planned', disembark_date: '2026-08-12' }, cruise: { disembark_date: '2026-08-01' }, want: false },
		{ name: 'cancelled past leg', leg: { status: 'cancelled', disembark_date: '2026-08-01' }, cruise: {}, want: false }
	] as const)('$name -> $want', ({ leg, cruise, want }) => {
		expect(isLegOverdue(leg, cruise, today)).toBe(want);
	});

	it('compares in Warsaw just after midnight, not in UTC', () => {
		// 00:30 on 10 Aug in Warsaw is still 9 Aug in UTC.
		const today = warsawToday(new Date('2026-08-09T22:30:00Z'));
		expect(isLegOverdue({ status: 'planned', disembark_date: '2026-08-09' }, null, today)).toBe(true);
	});
});

describe('legStatus', () => {
	it.each([
		[{ status: 'planned', disembark_date: '2026-08-09' }, 'overdue'],
		[{ status: 'planned', disembark_date: '2026-08-10' }, 'planned'],
		[{ status: 'planned' }, 'planned'],
		[{ status: 'cancelled', disembark_date: '2026-08-01' }, 'cancelled']
	] as const)('%j -> %s', (leg, want) => {
		expect(legStatus(leg, null, '2026-08-10')).toBe(want);
	});
});

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

	describe('past planned legs', () => {
		const today = '2026-11-13';
		const status = (trips: ReturnType<typeof makeTrip>[], cruise = makeCruise({ id: 1 }), voyages: ReturnType<typeof makeVoyage>[] = []) =>
			buildRejsEntries([cruise], trips, voyages, today)[0].status;

		it.each([
			{ name: 'a leg that ended yesterday', disembark: '2026-11-12', want: 'overdue' },
			{ name: 'a leg that ends today', disembark: '2026-11-13', want: 'upcoming' },
			{ name: 'a leg in progress', disembark: '2026-11-20', want: 'upcoming' }
		])('marks a cruise with $name as $want', ({ disembark, want }) => {
			expect(status([makeTrip({ cruise_id: 1, embark_date: '2026-11-10', disembark_date: disembark })])).toBe(want);
		});

		it('falls back to the cruise end when the leg has no dates', () => {
			const trip = makeTrip({ cruise_id: 1, embark_date: undefined, disembark_date: undefined });
			expect(status([trip], makeCruise({ id: 1, disembark_date: '2026-08-09' }))).toBe('overdue');
			expect(status([trip], makeCruise({ id: 1, disembark_date: undefined }))).toBe('upcoming');
		});

		it('stays overdue when other legs already sailed', () => {
			const trip = makeTrip({ cruise_id: 1, disembark_date: '2026-08-09' });
			expect(status([trip], makeCruise({ id: 1 }), [makeVoyage({ cruise_id: 1 })])).toBe('overdue');
		});

		it('ignores cancelled past legs', () => {
			const trip = makeTrip({ cruise_id: 1, status: 'cancelled', disembark_date: '2026-08-09' });
			expect(status([trip], makeCruise({ id: 1 }), [makeVoyage({ cruise_id: 1 })])).toBe('completed');
		});

		it('labels the status "Do rozliczenia"', () => {
			expect(rejsStatusLabels.overdue).toBe('Do rozliczenia');
		});
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

	it('moves a past cruise awaiting completion from upcoming to completed and all', () => {
		const today = '2026-11-13';
		const listed = buildRejsEntries(
			[
				makeCruise({ id: 1, name: 'Bornholm 2026', embark_date: '2026-08-01', disembark_date: '2026-08-09' }),
				makeCruise({ id: 2, name: 'Now', embark_date: '2026-11-10', disembark_date: '2026-11-20' }),
				makeCruise({ id: 3, name: 'Done', embark_date: '2026-06-01' })
			],
			[
				makeTrip({ id: 10, cruise_id: 1, embark_date: undefined, disembark_date: undefined }),
				makeTrip({ id: 20, cruise_id: 2, embark_date: '2026-11-10', disembark_date: '2026-11-20' })
			],
			[makeVoyage({ id: 30, cruise_id: 3 })],
			today
		);
		const pick = (tab: 'upcoming' | 'completed' | 'all') => filterRejsEntries(listed, tab).map((e) => e.name);
		expect(pick('upcoming')).toEqual(['Now']);
		expect(pick('completed')).toEqual(['Bornholm 2026', 'Done']);
		expect(pick('all')).toEqual(['Now', 'Bornholm 2026', 'Done']);
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

describe('rejsListUrl', () => {
	it.each([
		['upcoming', false, '/cruises'],
		['completed', false, '/cruises?tab=completed'],
		['upcoming', true, '/cruises?mine=1'],
		['all', true, '/cruises?tab=all&mine=1']
	] as const)('%s, mine=%s -> %s', (tab, mine, want) => {
		expect(rejsListUrl(tab, mine)).toBe(want);
	});
});

describe('isMineFilter', () => {
	it.each([
		['1', true],
		['0', false],
		[null, false],
		['true', false]
	])('%j -> %s', (value, want) => {
		expect(isMineFilter(value)).toBe(want);
	});
});

describe('onlyMine', () => {
	it('keeps only cruises the user took part in', () => {
		const entries = buildRejsEntries(
			[makeCruise({ id: 1, name: 'A' }), makeCruise({ id: 2, name: 'B' }), makeCruise({ id: 3, name: 'C' })],
			[],
			[]
		);
		expect(onlyMine(entries, new Set([1, 3])).map((e) => e.name)).toEqual(['A', 'C']);
		expect(onlyMine(entries, new Set())).toEqual([]);
	});
});
