import { describe, it, expect, beforeAll, afterAll, vi } from 'vitest';
import { makeCruise, makeSeries, makeTrip, makeVoyage } from '#lib/test-utils.js';
import {
	buildRejsEntries,
	filterRejsEntries,
	isLegOverdue,
	isMineFilter,
	legStatus,
	legStatusLabels,
	onlyMine,
	parseRejsTab,
	rejsListUrl,
	rejsStatusLabels,
	rejsTabs,
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
		{
			name: 'ended yesterday',
			leg: { status: 'planned', disembark_date: '2026-08-09' },
			want: true
		},
		{ name: 'ends today', leg: { status: 'planned', disembark_date: '2026-08-10' }, want: false },
		{
			name: 'ends tomorrow',
			leg: { status: 'planned', disembark_date: '2026-08-11' },
			want: false
		},
		{ name: 'no dates at all', leg: { status: 'planned' }, want: false },
		{
			name: 'cancelled past leg',
			leg: { status: 'cancelled', disembark_date: '2026-08-01' },
			want: false
		}
	] as const)('$name -> $want', ({ leg, want }) => {
		expect(isLegOverdue(leg, today)).toBe(want);
	});

	it('compares in Warsaw just after midnight, not in UTC', () => {
		// 00:30 on 10 Aug in Warsaw is still 9 Aug in UTC.
		const today = warsawToday(new Date('2026-08-09T22:30:00Z'));
		expect(isLegOverdue({ status: 'planned', disembark_date: '2026-08-09' }, today)).toBe(true);
	});
});

describe('default today', () => {
	it('flags a leg just after midnight in Warsaw while UTC is still on the previous day', () => {
		// 00:30 on 10 Aug in Warsaw (CEST) is 22:30 on 9 Aug in UTC.
		vi.setSystemTime(new Date('2026-08-09T22:30:00Z'));
		try {
			const leg = makeTrip({
				cruise_id: 1,
				embark_date: '2026-08-01',
				disembark_date: '2026-08-09'
			});
			expect(legStatus(leg)).toBe('overdue');
			expect(buildRejsEntries([makeCruise({ id: 1 })], [leg], [], [])[0].status).toBe('overdue');
		} finally {
			vi.setSystemTime(new Date('2025-05-01T12:00:00Z'));
		}
	});
});

describe('legStatus', () => {
	it.each([
		[{ status: 'planned', disembark_date: '2026-08-09' }, 'overdue'],
		[{ status: 'planned', disembark_date: '2026-08-10' }, 'planned'],
		[{ status: 'planned' }, 'planned'],
		[{ status: 'cancelled', disembark_date: '2026-08-01' }, 'cancelled']
	] as const)('%j -> %s', (leg, want) => {
		expect(legStatus(leg, '2026-08-10')).toBe(want);
	});
});

describe('upcoming labels', () => {
	it('match the Nadchodzące tab for legs and cruises', () => {
		expect(rejsTabs.find((t) => t.value === 'upcoming')?.label).toBe('Nadchodzące');
		expect(legStatusLabels.planned).toBe('Nadchodzący');
		expect(rejsStatusLabels.upcoming).toBe('Nadchodzący');
	});
});

describe('buildRejsEntries', () => {
	it.each([
		{ name: 'a published cruise', cruise: { published_at: 'x' }, trip: {}, series: 2, want: false },
		{ name: 'a hidden cruise', cruise: {}, trip: { published_at: 'x' }, series: 2, want: true },
		{
			name: 'a hidden leg of an edition',
			cruise: { published_at: 'x' },
			trip: {},
			series: 2,
			want: false
		},
		{
			name: 'a hidden standalone trip',
			cruise: { published_at: 'x' },
			trip: {},
			series: undefined,
			want: true
		}
	])('flags $name as hidden: $want', ({ cruise, trip, series, want }) => {
		const [entry] = buildRejsEntries(
			[makeCruise({ id: 7, series_id: series, ...cruise })],
			[makeTrip({ id: 4, cruise_id: 7, ...trip })],
			[],
			[]
		);
		expect(entry.hidden).toBe(want);
	});

	it('lists each cruise once with its legs summarized', () => {
		const entries = buildRejsEntries(
			[makeCruise({ id: 7, name: 'NNE 2025', enroll_token: 'tok' })],
			[makeTrip({ id: 1, cruise_id: 7 }), makeTrip({ id: 2, cruise_id: 7 })],
			[makeVoyage({ id: 3, cruise_id: 7 })],
			[]
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

	it.each([
		{
			name: 'a standalone trip links to the trip',
			cruise: makeCruise({ id: 7 }),
			trips: [makeTrip({ id: 4, cruise_id: 7 })],
			voyages: [],
			want: '/trips/4'
		},
		{
			name: 'an edition with one trip links to the edition',
			cruise: makeCruise({ id: 7, series_id: 2, edition: '2026' }),
			trips: [makeTrip({ id: 4, cruise_id: 7 })],
			voyages: [],
			want: '/cruises/7'
		},
		{
			name: 'a cruise without series with two trips links to the cruise',
			cruise: makeCruise({ id: 7 }),
			trips: [makeTrip({ id: 4, cruise_id: 7 }), makeTrip({ id: 5, cruise_id: 7 })],
			voyages: [],
			want: '/cruises/7'
		},
		{
			name: 'a cruise without series with a trip and a voyage links to the cruise',
			cruise: makeCruise({ id: 7 }),
			trips: [makeTrip({ id: 4, cruise_id: 7 })],
			voyages: [makeVoyage({ cruise_id: 7 })],
			want: '/cruises/7'
		},
		{
			name: 'a cruise without legs links to the cruise',
			cruise: makeCruise({ id: 7 }),
			trips: [],
			voyages: [],
			want: '/cruises/7'
		}
	])('$name', ({ cruise, trips, voyages, want }) => {
		const [entry] = buildRejsEntries([cruise], trips, voyages, []);
		expect(entry.href).toBe(want);
	});

	it("describes a standalone trip by the trip, not its wrapper cruise's copied fields", () => {
		const wrapper = makeCruise({ id: 7, name: 'Original', countries: 'PL, SE' });
		const edited = makeTrip({
			id: 4,
			cruise_id: 7,
			name: 'Renamed',
			embark_date: '2026-06-01',
			disembark_date: '2026-06-08',
			start_port: 'Gdańsk',
			end_port: 'Hel',
			countries: 'PL',
			max_crew: 5,
			enroll_token: 'leg'
		});
		const [entry] = buildRejsEntries([wrapper], [edited], [], [], '2026-11-15');
		expect(entry).toMatchObject({
			href: '/trips/4',
			name: 'Renamed',
			embarkDate: '2026-06-01',
			disembarkDate: '2026-06-08',
			startPort: 'Gdańsk',
			endPort: 'Hel',
			countries: 'PL',
			maxCrew: 5,
			enrollOpen: true,
			status: 'overdue',
			ended: true
		});
		expect(filterRejsEntries([entry], 'completed')).toHaveLength(1);
		expect(filterRejsEntries([entry], 'upcoming')).toHaveLength(0);
	});

	it("falls back to the wrapper cruise's countries when the standalone trip has none", () => {
		const wrapper = makeCruise({ id: 7, name: 'Wrapper', countries: 'PL, SE' });
		const trip = makeTrip({ id: 4, cruise_id: 7, name: 'Solo', countries: undefined });
		const [entry] = buildRejsEntries([wrapper], [trip], [], [], '2025-05-01');
		expect(entry).toMatchObject({ name: 'Solo', countries: 'PL, SE' });
	});

	it("shows an edition's countries but not its single leg's dates, ports or crew limit", () => {
		const edition = makeCruise({ id: 7, series_id: 2, name: 'Sycylia 2026', countries: 'IT' });
		const leg = makeTrip({
			id: 4,
			cruise_id: 7,
			embark_date: '2026-06-01',
			disembark_date: '2026-06-08',
			start_port: 'Palermo',
			end_port: 'Trapani',
			countries: 'MT',
			max_crew: 5
		});
		const [entry] = buildRejsEntries([edition], [leg], [], [makeSeries({ id: 2 })], '2026-05-01');
		expect(entry).toMatchObject({
			href: '/cruises/7',
			name: 'Sycylia 2026',
			countries: 'IT',
			sortDate: '2026-06-01'
		});
		expect(entry.embarkDate).toBeUndefined();
		expect(entry.disembarkDate).toBeUndefined();
		expect(entry.startPort).toBeUndefined();
		expect(entry.endPort).toBeUndefined();
		expect(entry.maxCrew).toBeUndefined();
	});

	it.each([
		{ name: 'an inland series', seriesId: 2, want: 'inland' },
		{ name: 'a sea series', seriesId: 3, want: 'sea' },
		{ name: 'no series', seriesId: undefined, want: 'sea' },
		{ name: 'a series missing from the list', seriesId: 9, want: 'sea' }
	])('takes the kind of a cruise in $name: $want', ({ seriesId, want }) => {
		const series = [makeSeries({ id: 2, kind: 'inland' }), makeSeries({ id: 3, kind: 'sea' })];
		const [entry] = buildRejsEntries([makeCruise({ id: 7, series_id: seriesId })], [], [], series);
		expect(entry.kind).toBe(want);
	});

	it('ignores legs of other cruises', () => {
		const [entry] = buildRejsEntries(
			[makeCruise({ id: 1 })],
			[makeTrip({ cruise_id: 2 })],
			[makeVoyage({ cruise_id: 2, miles: 99 })],
			[]
		);
		expect(entry).toMatchObject({ plannedLegs: 0, completedLegs: 0, miles: 0 });
	});

	it.each([
		{
			name: 'all legs sailed',
			trips: [],
			voyages: [makeVoyage({ cruise_id: 1 })],
			want: 'completed'
		},
		{
			name: 'a leg still planned',
			trips: [makeTrip({ cruise_id: 1 })],
			voyages: [makeVoyage({ cruise_id: 1 })],
			want: 'upcoming'
		},
		{
			name: 'only cancelled legs',
			trips: [makeTrip({ cruise_id: 1, status: 'cancelled' })],
			voyages: [],
			want: 'cancelled'
		},
		{
			name: 'cancelled leg plus a sailed one',
			trips: [makeTrip({ cruise_id: 1, status: 'cancelled' })],
			voyages: [makeVoyage({ cruise_id: 1 })],
			want: 'completed'
		},
		{ name: 'no legs yet', trips: [], voyages: [], want: 'upcoming' }
	])('marks a cruise with $name as $want', ({ trips, voyages, want }) => {
		const [entry] = buildRejsEntries([makeCruise({ id: 1 })], trips, voyages, []);
		expect(entry.status).toBe(want);
	});

	describe('past planned legs', () => {
		const today = '2026-11-13';
		const status = (
			trips: ReturnType<typeof makeTrip>[],
			cruise = makeCruise({ id: 1 }),
			voyages: ReturnType<typeof makeVoyage>[] = []
		) => buildRejsEntries([cruise], trips, voyages, [], today)[0].status;

		it.each([
			{ name: 'a leg that ended yesterday', disembark: '2026-11-12', want: 'overdue' },
			{ name: 'a leg that ends today', disembark: '2026-11-13', want: 'upcoming' },
			{ name: 'a leg in progress', disembark: '2026-11-20', want: 'upcoming' }
		])('marks a cruise with $name as $want', ({ disembark, want }) => {
			expect(
				status([makeTrip({ cruise_id: 1, embark_date: '2026-11-10', disembark_date: disembark })])
			).toBe(want);
		});

		it('keeps a cruise upcoming when its leg has no dates', () => {
			const trip = makeTrip({ cruise_id: 1, embark_date: undefined, disembark_date: undefined });
			expect(status([trip])).toBe('upcoming');
		});

		it('stays overdue when other legs already sailed', () => {
			const trip = makeTrip({ cruise_id: 1, disembark_date: '2026-08-09' });
			expect(status([trip], makeCruise({ id: 1 }), [makeVoyage({ cruise_id: 1 })])).toBe('overdue');
		});

		it('ignores cancelled past legs', () => {
			const trip = makeTrip({ cruise_id: 1, status: 'cancelled', disembark_date: '2026-08-09' });
			expect(status([trip], makeCruise({ id: 1 }), [makeVoyage({ cruise_id: 1 })])).toBe(
				'completed'
			);
		});

		it('labels the status "Do rozliczenia"', () => {
			expect(rejsStatusLabels.overdue).toBe('Do rozliczenia');
		});
	});

	it('sums hours and miles over the sailed legs', () => {
		const [entry] = buildRejsEntries(
			[makeCruise({ id: 1 })],
			[],
			[
				makeVoyage({ cruise_id: 1, miles: 100, hours_total: 30 }),
				makeVoyage({ id: 2, cruise_id: 1, miles: 50, hours_total: 12 })
			],
			[]
		);
		expect(entry).toMatchObject({ miles: 150, hoursTotal: 42 });
	});

	it('sorts by the earliest leg embark date over trips and voyages', () => {
		const [entry] = buildRejsEntries(
			[makeCruise({ id: 1, series_id: 2 })],
			[
				makeTrip({ cruise_id: 1, embark_date: '2026-08-10' }),
				makeTrip({ id: 2, cruise_id: 1, embark_date: undefined })
			],
			[makeVoyage({ cruise_id: 1, embark_date: '2026-07-01' })],
			[]
		);
		expect(entry.sortDate).toBe('2026-07-01');
	});

	it('has no sort date without dated legs', () => {
		const [entry] = buildRejsEntries([makeCruise({ id: 1 })], [], [], []);
		expect(entry.sortDate).toBeUndefined();
	});
});

describe('filterRejsEntries', () => {
	const leg = (id: number, cruiseId: number, embark: string) =>
		makeTrip({ id, cruise_id: cruiseId, embark_date: embark, disembark_date: embark });
	const entries = buildRejsEntries(
		[
			makeCruise({ series_id: 1, id: 1, name: 'Late' }),
			makeCruise({ series_id: 1, id: 2, name: 'Soon' }),
			makeCruise({ series_id: 1, id: 3, name: 'Undated' }),
			makeCruise({ series_id: 1, id: 4, name: 'Cancelled' }),
			makeCruise({ series_id: 1, id: 5, name: 'Old' }),
			makeCruise({ series_id: 1, id: 6, name: 'Recent' })
		],
		[
			leg(10, 1, '2026-09-01'),
			leg(20, 2, '2026-05-01'),
			{ ...leg(40, 4, '2026-06-01'), status: 'cancelled' }
		],
		[
			makeVoyage({ id: 50, cruise_id: 5, embark_date: '2020-07-01' }),
			makeVoyage({ id: 60, cruise_id: 6, embark_date: '2025-07-01' })
		],
		[],
		'2025-05-01'
	);
	const names = (tab: 'upcoming' | 'completed' | 'all') =>
		filterRejsEntries(entries, tab).map((e) => e.name);

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
				makeCruise({ series_id: 1, id: 1, name: 'Bornholm 2026' }),
				makeCruise({ series_id: 1, id: 2, name: 'Now' }),
				makeCruise({ series_id: 1, id: 3, name: 'Done' })
			],
			[
				makeTrip({ id: 10, cruise_id: 1, embark_date: '2026-08-01', disembark_date: '2026-08-09' }),
				makeTrip({ id: 20, cruise_id: 2, embark_date: '2026-11-10', disembark_date: '2026-11-20' })
			],
			[
				makeVoyage({
					id: 30,
					cruise_id: 3,
					embark_date: '2026-06-01',
					disembark_date: '2026-06-08'
				})
			],
			[],
			today
		);
		const pick = (tab: 'upcoming' | 'completed' | 'all') =>
			filterRejsEntries(listed, tab).map((e) => e.name);
		expect(pick('upcoming')).toEqual(['Now']);
		expect(pick('completed')).toEqual(['Bornholm 2026', 'Done']);
		expect(pick('all')).toEqual(['Now', 'Bornholm 2026', 'Done']);
	});

	it('keeps a cruise still at sea upcoming by its latest leg while flagging its past leg', () => {
		const listed = buildRejsEntries(
			[makeCruise({ id: 1, name: 'Baltic' })],
			[
				makeTrip({ id: 10, cruise_id: 1, embark_date: '2026-11-01', disembark_date: '2026-11-07' }),
				makeTrip({ id: 11, cruise_id: 1, embark_date: '2026-11-10', disembark_date: '2026-11-20' })
			],
			[],
			[],
			'2026-11-13'
		);
		expect(listed[0]).toMatchObject({ status: 'overdue', ended: false });
		expect(filterRejsEntries(listed, 'upcoming').map((e) => e.name)).toEqual(['Baltic']);
		expect(filterRejsEntries(listed, 'completed')).toEqual([]);
	});

	it("moves a past leg's cruise to completed once its last leg ended", () => {
		const listed = buildRejsEntries(
			[makeCruise({ id: 1, name: 'Baltic' })],
			[makeTrip({ id: 10, cruise_id: 1, embark_date: '2026-11-01', disembark_date: '2026-11-07' })],
			[makeVoyage({ id: 20, cruise_id: 1, disembark_date: '2026-11-12' })],
			[],
			'2026-11-13'
		);
		expect(filterRejsEntries(listed, 'completed').map((e) => e.name)).toEqual(['Baltic']);
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
			[
				makeCruise({ id: 1, name: 'A' }),
				makeCruise({ id: 2, name: 'B' }),
				makeCruise({ id: 3, name: 'C' })
			],
			[],
			[],
			[]
		);
		expect(onlyMine(entries, new Set([1, 3])).map((e) => e.name)).toEqual(['A', 'C']);
		expect(onlyMine(entries, new Set())).toEqual([]);
	});
});
