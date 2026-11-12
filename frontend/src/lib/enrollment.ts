export const statusLabels: Record<string, string> = {
	pending: 'oczekujący',
	accepted: 'zaakceptowany',
	rejected: 'odrzucony',
	waitlisted: 'lista rezerwowa'
};

interface LegTrip {
	id: number;
	embark_date?: string;
}

/**
 * numberLegs orders a cruise's trips by embark date (undated last, then by
 * id) and numbers them from 1. Cancelled trips keep their number so the
 * remaining legs do not shift when one is cancelled.
 */
export function numberLegs<T extends LegTrip>(trips: readonly T[]): { number: number; trip: T }[] {
	return [...trips]
		.sort((a, b) => {
			const da = a.embark_date ?? '';
			const db = b.embark_date ?? '';
			if (da !== db) {
				if (!da) return 1;
				if (!db) return -1;
				return da < db ? -1 : 1;
			}
			return a.id - b.id;
		})
		.map((trip, i) => ({ number: i + 1, trip }));
}
