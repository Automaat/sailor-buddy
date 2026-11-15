import { describe, it, expect, beforeAll, afterAll, vi } from 'vitest';
import { makeTrip } from '#lib/test-utils.js';
import {
	isLegOverdue,
	isStandaloneHidden,
	legStatus,
	standaloneHref,
	standaloneStatus,
	standaloneStatusLabels,
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

describe('standalone entries', () => {
	const published = '2026-01-01T00:00:00Z';

	it.each([
		['trip', '/trips/7'],
		['voyage', '/voyages/7'],
		['cruise', '/cruises/7']
	] as const)('links a %s to %s', (kind, href) => {
		expect(standaloneHref({ kind, id: 7 })).toBe(href);
	});

	it.each([
		['published entry of a published cruise', published, published, false],
		['hidden entry', undefined, published, true],
		['entry of a hidden cruise', published, undefined, true]
	])('%s -> hidden %s', (_, ownPublished, cruisePublished, want) => {
		expect(
			isStandaloneHidden({ published_at: ownPublished, cruise_published_at: cruisePublished })
		).toBe(want);
	});

	it.each([
		[{ kind: 'trip', cancelled: false, disembark_date: '2026-08-10' }, 'planned'],
		[{ kind: 'trip', cancelled: false, disembark_date: '2026-08-09' }, 'overdue'],
		[{ kind: 'trip', cancelled: true, disembark_date: '2026-08-09' }, 'cancelled'],
		[{ kind: 'trip', cancelled: false }, 'planned'],
		[{ kind: 'voyage', cancelled: false, disembark_date: '2026-08-09' }, 'completed'],
		[{ kind: 'cruise', cancelled: false }, null]
	] as const)('%j -> %s', (entry, want) => {
		expect(standaloneStatus(entry, '2026-08-10')).toBe(want);
	});

	it('labels a sailed voyage as completed', () => {
		expect(standaloneStatusLabels.completed).toBe('Zrealizowany');
		expect(standaloneStatusLabels.overdue).toBe('Do rozliczenia');
	});
});
