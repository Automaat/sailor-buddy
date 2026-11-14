import { formatDate } from './dates.js';

export const statusLabels: Record<string, string> = {
	pending: 'oczekujący',
	accepted: 'zaakceptowany',
	rejected: 'odrzucony',
	waitlisted: 'lista rezerwowa'
};

/** PaymentStatus is the manual payment status an admin sets on a leg sign-up. */
export type PaymentStatus = 'unpaid' | 'deposit' | 'paid';

/** paymentStatuses lists the payment statuses in their natural order. */
export const paymentStatuses: readonly PaymentStatus[] = ['unpaid', 'deposit', 'paid'];

export const paymentLabels: Record<PaymentStatus, string> = {
	unpaid: 'Brak wpłaty',
	deposit: 'Zaliczka wpłacona',
	paid: 'Opłacone'
};

/**
 * paymentCounts counts sign-ups per payment status. Sign-ups without a
 * payment status count as unpaid.
 */
export function paymentCounts(
	enrollments: readonly { payment_status?: PaymentStatus }[]
): Record<PaymentStatus, number> {
	const counts: Record<PaymentStatus, number> = { unpaid: 0, deposit: 0, paid: 0 };
	for (const e of enrollments) counts[e.payment_status ?? 'unpaid']++;
	return counts;
}

/**
 * freePlaces is a trip's crew limit minus its accepted enrollments, never
 * below zero. It returns null when the trip has no crew limit; the trip forms
 * store 0 for "no limit". Sign-ups stay open when a trip is full: they go on
 * the waiting list, and admins promote from it by hand.
 */
export function freePlaces(maxCrew: number | null | undefined, accepted: number): number | null {
	if (!maxCrew) return null;
	return Math.max(0, maxCrew - accepted);
}

/**
 * freePlacesLabel renders a leg's free places as "wolne 5 z 8", or null when
 * the leg has no crew limit.
 */
export function freePlacesLabel(
	maxCrew: number | null | undefined,
	accepted: number
): string | null {
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

/**
 * waitlistLabel renders a waitlisted sign-up's status with its position, as
 * "lista rezerwowa (2.)", or the plain status label when the position is
 * unknown.
 */
export function waitlistLabel(position: number | null | undefined): string {
	return position ? `${statusLabels.waitlisted} (${position}.)` : statusLabels.waitlisted;
}

interface QueuedEnrollment {
	id: number;
	status: string;
	created_at: string;
}

/**
 * nextWaitlisted returns the id of the waitlisted enrollment an admin should
 * promote next: the earliest by sign-up time (ties by id), but only while the
 * leg has a free place. It returns null when the leg is full, has no crew
 * limit, or nobody is waitlisted. Promotion itself stays a manual accept.
 */
export function nextWaitlisted(
	enrollments: readonly QueuedEnrollment[],
	maxCrew: number | null | undefined,
	accepted: number
): number | null {
	const free = freePlaces(maxCrew, accepted);
	if (!free) return null;
	// Compare instants, not strings: the API drops trailing zeros from
	// fractional seconds, so ".5Z" sorts after ".55Z" as text.
	const waiting = enrollments
		.filter((e) => e.status === 'waitlisted')
		.map((e) => ({ id: e.id, at: Date.parse(e.created_at) }))
		.sort((a, b) => a.at - b.at || a.id - b.id);
	return waiting[0]?.id ?? null;
}

/**
 * signUpClosedByDeadline is true once the club date today (YYYY-MM-DD,
 * Europe/Warsaw, see warsawToday) is after the leg's sign-up deadline. The
 * deadline day itself is still open; a leg without a deadline never closes.
 */
export function signUpClosedByDeadline(
	deadline: string | null | undefined,
	today: string
): boolean {
	return !!deadline && today > deadline;
}

/**
 * signUpDeadlineLabel renders a sign-up deadline as "Zapisy do 10 maj 2027",
 * or null when the leg has none.
 */
export function signUpDeadlineLabel(deadline: string | null | undefined): string | null {
	return deadline ? `Zapisy do ${formatDate(deadline)}` : null;
}
