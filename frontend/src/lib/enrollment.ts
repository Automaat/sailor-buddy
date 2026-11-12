export const statusLabels: Record<string, string> = {
	pending: 'oczekujący',
	accepted: 'zaakceptowany',
	rejected: 'odrzucony',
	waitlisted: 'lista rezerwowa'
};

/**
 * freePlaces is a trip's crew limit minus its accepted enrollments, never
 * below zero. It returns null when the trip has no crew limit. Enrollment
 * stays open when a trip is full: admins decide who sails.
 */
export function freePlaces(maxCrew: number | null | undefined, accepted: number): number | null {
	if (maxCrew == null) return null;
	return Math.max(0, maxCrew - accepted);
}
