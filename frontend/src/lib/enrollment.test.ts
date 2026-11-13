import { describe, it, expect } from 'vitest';
import { freePlacesLabel, numberLegs, paymentCounts, signUpClosedByDeadline, signUpDeadlineLabel } from './enrollment';

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

describe('signUpClosedByDeadline', () => {
	it.each([
		['2027-05-10', '2027-05-09', false],
		['2027-05-10', '2027-05-10', false],
		['2027-05-10', '2027-05-11', true],
		[undefined, '2099-01-01', false],
		[null, '2099-01-01', false]
	] as const)('deadline %s on %s is closed: %s', (deadline, today, want) => {
		expect(signUpClosedByDeadline(deadline, today)).toBe(want);
	});
});

describe('signUpDeadlineLabel', () => {
	it('formats the date the Polish way', () => {
		expect(signUpDeadlineLabel('2027-05-10')).toBe('Zapisy do 10.05.2027');
	});

	it('is null without a deadline', () => {
		expect(signUpDeadlineLabel(undefined)).toBeNull();
		expect(signUpDeadlineLabel('')).toBeNull();
	});
});

describe('paymentCounts', () => {
	it('counts each payment status, treating a missing one as unpaid', () => {
		expect(
			paymentCounts([
				{ payment_status: 'paid' },
				{ payment_status: 'deposit' },
				{ payment_status: 'deposit' },
				{ payment_status: 'unpaid' },
				{}
			])
		).toEqual({ unpaid: 2, deposit: 2, paid: 1 });
	});

	it('returns zeros for no sign-ups', () => {
		expect(paymentCounts([])).toEqual({ unpaid: 0, deposit: 0, paid: 0 });
	});
});
