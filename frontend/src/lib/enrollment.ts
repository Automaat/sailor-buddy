export const statusLabels: Record<string, string> = {
	pending: 'oczekujący',
	accepted: 'zaakceptowany',
	rejected: 'odrzucony',
	waitlisted: 'lista rezerwowa'
};

/**
 * freePlaces is a trip's crew limit minus its accepted enrollments, never
 * below zero. It returns null when the trip has no crew limit; the trip forms
 * store 0 for "no limit". Enrollment stays open when a trip is full: admins
 * decide who sails.
 */
export function freePlaces(maxCrew: number | null | undefined, accepted: number): number | null {
	if (!maxCrew) return null;
	return Math.max(0, maxCrew - accepted);
}

/**
 * freePlacesLabel renders a leg's free places as "wolne 5 z 8", or null when
 * the leg has no crew limit.
 */
export function freePlacesLabel(maxCrew: number | null | undefined, accepted: number): string | null {
	const free = freePlaces(maxCrew, accepted);
	return free === null ? null : `wolne ${free} z ${maxCrew}`;
}

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
