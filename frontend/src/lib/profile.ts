import type { components } from '#lib/api/schema.js';
import type { User } from '#lib/api/types.js';

type UpdateProfileBody = components['schemas']['UpdateProfileBody'];

export type PatentType = NonNullable<UpdateProfileBody['patent_type']>;

export const PATENT_LABELS: Record<PatentType, string> = {
	zeglarz_jachtowy: 'Żeglarz Jachtowy',
	jachtowy_sternik_morski: 'Jachtowy Sternik Morski',
	kapitan_jachtowy: 'Kapitan Jachtowy'
};

export const PATENT_OPTIONS: { value: '' | PatentType; label: string }[] = [
	{ value: '', label: 'Nie mam patentu' },
	...(Object.entries(PATENT_LABELS) as [PatentType, string][]).map(([value, label]) => ({
		value,
		label
	}))
];

export type ProfileForm = {
	firstName: string;
	lastName: string;
	phone: string;
	patentType: '' | PatentType;
	patentNumber: string;
};

/** splitName guesses given and family name from a single display name. */
export function splitName(name: string): { firstName: string; lastName: string } {
	const [firstName = '', ...rest] = name.trim().split(/\s+/);
	return { firstName, lastName: rest.join(' ') };
}

/**
 * profileFormFromUser seeds the profile form, falling back to the display
 * name for accounts that never filled in the structured name.
 */
export function profileFormFromUser(user: User | null, displayName = ''): ProfileForm {
	const fallback = splitName(user?.name || displayName);
	const hasSplitName = !!(user?.first_name || user?.last_name);
	return {
		firstName: hasSplitName ? (user?.first_name ?? '') : fallback.firstName,
		lastName: hasSplitName ? (user?.last_name ?? '') : fallback.lastName,
		phone: user?.phone ?? '',
		patentType: (user?.patent_type as PatentType | undefined) ?? '',
		patentNumber: user?.patent_number ?? ''
	};
}

/** profileBody maps the form onto the API payload, omitting blank optionals. */
export function profileBody(form: ProfileForm): UpdateProfileBody {
	const patentType = form.patentType || undefined;
	return {
		first_name: form.firstName.trim(),
		last_name: form.lastName.trim(),
		phone: form.phone.trim() || undefined,
		patent_type: patentType,
		patent_number: patentType ? form.patentNumber.trim() || undefined : undefined
	};
}

/** patentSummary renders a patent as "Grade · number" for read-only views. */
export function patentSummary(type?: string | null, number?: string | null): string {
	if (!type) return 'brak patentu';
	const label = PATENT_LABELS[type as PatentType] ?? type;
	return number ? `${label} · ${number}` : label;
}
