import { isAdminOrCaptainGroup } from '#lib/access.js';
import { plural } from '#lib/plural.js';

/**
 * canSeeHidden mirrors the API rule: admins and the captain group read hidden
 * drafts. Members never get them, so the badge stays off for them even if a
 * response lacked published_at.
 */
export const canSeeHidden = isAdminOrCaptainGroup;

type Publishable = { published_at?: string | null };

/** isHidden is true for a draft: a record that has no publication time yet. */
export function isHidden(record: Publishable): boolean {
	return !record.published_at;
}

/**
 * isLegHidden is true when members cannot see the leg: it or its cruise is
 * still a draft.
 */
export function isLegHidden(leg: Publishable, cruise: Publishable | null | undefined): boolean {
	return isHidden(leg) || (!!cruise && isHidden(cruise));
}

/** The question asked before publishing a cruise that still has hidden legs. */
export function publishLegsQuestion(hiddenLegs: number): string {
	const noun = plural(hiddenLegs, 'ukryty etap', 'ukryte etapy', 'ukrytych etapów');
	return `Opublikować też ${hiddenLegs} ${noun}?`;
}
