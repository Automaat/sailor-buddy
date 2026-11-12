import { describe, it, expect } from 'vitest';
import { numberLegs } from './enrollment';

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
