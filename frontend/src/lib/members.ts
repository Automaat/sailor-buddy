import type { Member } from '#lib/api/types.js';

export type MemberFilter = 'all' | Member['membership'] | 'captain' | 'incomplete';

export const memberFilters: { value: MemberFilter; label: string }[] = [
	{ value: 'all', label: 'Wszyscy' },
	{ value: 'czlonek', label: 'Członek' },
	{ value: 'sympatyk', label: 'Sympatyk' },
	{ value: 'gosc', label: 'Gość' },
	{ value: 'captain', label: 'Kapitanowie' },
	{ value: 'incomplete', label: 'Profil niekompletny' }
];

/** fold lowercases text and strips Polish diacritics so "gosc" finds "Gość". */
export function fold(text: string): string {
	return text
		.toLocaleLowerCase('pl')
		.normalize('NFD')
		.replace(/\p{M}/gu, '')
		.replace(/ł/g, 'l');
}

export function matchesFilter(member: Member, filter: MemberFilter): boolean {
	switch (filter) {
		case 'all':
			return true;
		case 'captain':
			return member.is_captain;
		case 'incomplete':
			return !member.onboarded;
		default:
			return member.membership === filter;
	}
}

export function matchesQuery(member: Member, query: string): boolean {
	const q = fold(query.trim());
	if (!q) return true;
	return fold(member.name).includes(q) || fold(member.email).includes(q);
}

/** filterMembers applies the search query and the selected chip. */
export function filterMembers(members: Member[], query: string, filter: MemberFilter): Member[] {
	return members.filter((m) => matchesFilter(m, filter) && matchesQuery(m, query));
}

/** filterCounts counts members per chip over the whole list. */
export function filterCounts(members: Member[]): Record<MemberFilter, number> {
	const counts = Object.fromEntries(memberFilters.map((f) => [f.value, 0])) as Record<
		MemberFilter,
		number
	>;
	for (const m of members) {
		for (const f of memberFilters) {
			if (matchesFilter(m, f.value)) counts[f.value]++;
		}
	}
	return counts;
}
