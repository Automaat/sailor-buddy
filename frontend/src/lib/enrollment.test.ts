import { describe, it, expect } from 'vitest';
import { freePlaces, freePlacesLabel, nextWaitlisted, numberLegs, waitlistLabel } from './enrollment';

describe('numberLegs', () => {
	it('orders by embark date, undated trips last, ties by id', () => {
		const legs = numberLegs([
			{ id: 5, embark_date: undefined },
			{ id: 4, embark_date: '2025-07-10' },
			{ id: 3, embark_date: '2025-07-01' },
			{ id: 2, embark_date: '2025-07-10' },
			{ id: 1, embark_date: undefined }
		]);
		expect(legs.map((l) => [l.number, l.trip.id])).toEqual([
			[1, 3],
			[2, 2],
			[3, 4],
			[4, 1],
			[5, 5]
		]);
	});

	it('does not reorder the input array', () => {
		const trips = [
			{ id: 2, embark_date: '2025-07-10' },
			{ id: 1, embark_date: '2025-07-01' }
		];
		numberLegs(trips);
		expect(trips.map((t) => t.id)).toEqual([2, 1]);
	});
});

describe('freePlacesLabel', () => {
	it.each([
		[8, 3, 'wolne 5 z 8'],
		[8, 8, 'wolne 0 z 8'],
		[8, 10, 'wolne 0 z 8'],
		[0, 0, null],
		[undefined, 2, null]
	] as const)('max %s with %s accepted is %s', (max, accepted, want) => {
		expect(freePlacesLabel(max, accepted)).toBe(want);
	});
});

describe('freePlaces', () => {
	it.each([
		[8, 3, 5],
		[8, 8, 0],
		[8, 11, 0],
		[1, 2, 0],
		[0, 4, null],
		[null, 4, null],
		[undefined, 0, null]
	] as const)('max %s with %s accepted is %s, never below zero', (max, accepted, want) => {
		expect(freePlaces(max, accepted)).toBe(want);
	});
});

describe('waitlistLabel', () => {
	it('shows the position when known', () => {
		expect(waitlistLabel(2)).toBe('lista rezerwowa (2.)');
	});

	it.each([null, undefined, 0])('falls back to the status for %s', (pos) => {
		expect(waitlistLabel(pos)).toBe('lista rezerwowa');
	});
});

describe('nextWaitlisted', () => {
	const rows = [
		{ id: 4, status: 'waitlisted', created_at: '2026-01-03T00:00:00Z' },
		{ id: 1, status: 'accepted', created_at: '2026-01-01T00:00:00Z' },
		{ id: 3, status: 'waitlisted', created_at: '2026-01-02T00:00:00Z' },
		{ id: 2, status: 'waitlisted', created_at: '2026-01-02T00:00:00Z' }
	];

	it('picks the earliest waitlisted sign-up, ties by id, when a place is free', () => {
		expect(nextWaitlisted(rows, 2, 1)).toBe(2);
	});

	it('orders by instant, not by text', () => {
		const sameSecond = [
			{ id: 8, status: 'waitlisted', created_at: '2026-01-01T10:00:00.55Z' },
			{ id: 9, status: 'waitlisted', created_at: '2026-01-01T10:00:00.5Z' }
		];
		expect(nextWaitlisted(sameSecond, 2, 1)).toBe(9);
	});

	it('marks no one while the leg is full', () => {
		expect(nextWaitlisted(rows, 2, 2)).toBeNull();
		expect(nextWaitlisted(rows, 2, 3)).toBeNull();
	});

	it('marks no one without a crew limit or a waiting list', () => {
		expect(nextWaitlisted(rows, 0, 1)).toBeNull();
		expect(nextWaitlisted([rows[1]], 4, 1)).toBeNull();
	});
});
