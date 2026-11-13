import type { CrewMember } from '#lib/api/types.js';
import { fold } from '#lib/members.js';

export type AccountFilter = 'all' | 'with' | 'without';

export const accountFilters: { value: AccountFilter; label: string }[] = [
	{ value: 'all', label: 'Wszyscy' },
	{ value: 'with', label: 'Z kontem' },
	{ value: 'without', label: 'Bez konta' }
];

export function matchesAccount(person: CrewMember, filter: AccountFilter): boolean {
	if (filter === 'all') return true;
	return person.has_account === (filter === 'with');
}

/** matchesPerson finds a person by name, any other name or e-mail, ignoring case and Polish diacritics. */
export function matchesPerson(person: CrewMember, query: string): boolean {
	const q = fold(query.trim());
	if (!q) return true;
	const fields = [person.full_name, person.email ?? '', ...(person.other_names ?? [])];
	return fields.some((field) => fold(field).includes(q));
}

/** parseOtherNames splits a comma- or newline-separated list into trimmed, non-empty names. */
export function parseOtherNames(text: string): string[] {
	return text
		.split(/[,\n]/)
		.map((name) => name.trim())
		.filter((name) => name !== '');
}
