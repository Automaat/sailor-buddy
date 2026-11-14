import { redirect } from '@sveltejs/kit';
import { getTrip, getTripSignUp, listTripEnrollments } from '#lib/api/routes.js';
import { ApiError } from '#lib/api/client.js';
import { noteFailure, settle } from '#lib/api/errors.js';
import { found } from '#lib/api/load.js';
import type { LegSignUp, TripEnrollment } from '#lib/api/types.js';
import type { PageLoad } from './$types';

function countAccepted(list: TripEnrollment[]) {
	return list.filter((e) => e.status === 'accepted').length;
}

/**
 * Accepted crew the list does not show: cruise enrollments assigned to this
 * leg. They take places too, so free places add them back. Without the leg's
 * sign-up summary it is 0 and the banner says the count did not load.
 */
function otherAcceptedCount(signUp: LegSignUp | null, enrollments: TripEnrollment[]) {
	return signUp ? Math.max(0, signUp.accepted_count - countAccepted(enrollments)) : 0;
}

/**
 * Sends a member back to the trip page, since the API refuses the enrollment
 * list to anyone but an admin; passes any other failure on.
 */
function adminOnly(tripId: number) {
	return (e: unknown): never => {
		if (e instanceof ApiError && e.status === 403) redirect(307, `/trips/${tripId}`);
		throw e;
	};
}

export const load: PageLoad = async ({ params }) => {
	const id = Number(params.id);
	const trip = await found(getTrip(id), {
		title: 'Nie znaleziono rejsu',
		message: 'Ten rejs nie istnieje lub nie masz do niego dostępu.',
		backHref: '/cruises',
		backLabel: 'Wszystkie rejsy'
	});
	const loadErrors: string[] = [];
	const [enrollments, signUp] = await Promise.all([
		listTripEnrollments(id).then((e) => e ?? [], adminOnly(id)),
		settle(getTripSignUp(id), null, noteFailure(loadErrors, 'liczba zajętych miejsc'))
	]);
	return { trip, enrollments, otherAccepted: otherAcceptedCount(signUp, enrollments), loadErrors };
};
