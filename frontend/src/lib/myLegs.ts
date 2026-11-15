import type { MyLeg } from '#lib/api/types.js';

type SignUpStatus = NonNullable<MyLeg['sign_up_status']>;

const signUpLabels: Record<SignUpStatus, string> = {
	pending: 'Zapis: oczekuje',
	accepted: 'Zapis: zaakceptowany',
	waitlisted: 'Zapis: lista rezerwowa'
};

/** myLegRelationLabel names how the caller takes part in the leg. */
export function myLegRelationLabel(leg: Pick<MyLeg, 'relation' | 'sign_up_status'>): string {
	if (leg.relation === 'captain') return 'Kapitan';
	if (leg.relation === 'crew') return 'W załodze';
	return signUpLabels[leg.sign_up_status ?? 'pending'];
}

/** myLegRelationBadge is the badge colour: confirmed places stand out from sign-ups still in review. */
export function myLegRelationBadge(leg: Pick<MyLeg, 'relation' | 'sign_up_status'>): string {
	if (leg.relation === 'captain') return 'bg-[var(--navy)] text-white';
	if (leg.relation === 'crew' || leg.sign_up_status === 'accepted')
		return 'bg-[var(--ocean)] text-white';
	return 'bg-[var(--ocean-tint)] text-[var(--ocean)]';
}

/**
 * myLegEditionLabel names the event a leg belongs to: "Sycylia · edycja
 * 2027" for an edition of a series, or the cruise name for a leg without one.
 */
export function myLegEditionLabel(
	leg: Pick<MyLeg, 'series_name' | 'edition' | 'cruise_name'>
): string {
	if (!leg.series_name) return leg.cruise_name;
	return leg.edition ? `${leg.series_name} · edycja ${leg.edition}` : leg.series_name;
}
